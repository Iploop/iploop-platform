const jwt = require('jsonwebtoken');
const { query, getCache, setCache } = require('../config/database');
const { APIError } = require('./errorHandler');
const logger = require('../utils/logger');

// JWT authentication middleware
async function authenticateToken(req, res, next) {
  try {
    const authHeader = req.headers['authorization'];
    const token = authHeader && authHeader.split(' ')[1];

    if (!token) {
      throw new APIError('Access token required', 401);
    }

    // Check if token is blacklisted
    const blacklisted = await getCache(`blacklist:${token}`);
    if (blacklisted) {
      throw new APIError('Token has been revoked', 401);
    }

    // Verify token
    const decoded = jwt.verify(token, process.env.JWT_SECRET);
    
    // Check cache for user data
    let user = await getCache(`user:${decoded.userId}`);
    
    if (!user) {
      // Fetch user from database
      const result = await query(
        'SELECT id, email, first_name, last_name, status, role FROM users WHERE id = $1',
        [decoded.userId]
      );

      if (result.rows.length === 0) {
        throw new APIError('User not found', 401);
      }

      user = result.rows[0];
      
      if (user.status !== 'active') {
        throw new APIError('Account is not active', 401);
      }

      // Cache user data for 15 minutes
      await setCache(`user:${decoded.userId}`, user, 900);
    }

    // Check if user status is active
    if (user.status !== 'active') {
      throw new APIError('Account is not active', 401);
    }

    req.user = user;
    req.token = token;
    next();
  } catch (error) {
    if (error.name === 'JsonWebTokenError') {
      next(new APIError('Invalid token', 401));
    } else if (error.name === 'TokenExpiredError') {
      next(new APIError('Token expired', 401));
    } else {
      next(error);
    }
  }
}

// Optional authentication (for endpoints that work with or without auth)
async function optionalAuth(req, res, next) {
  try {
    const authHeader = req.headers['authorization'];
    if (authHeader) {
      await authenticateToken(req, res, next);
    } else {
      next();
    }
  } catch (error) {
    // Don't fail on optional auth errors, just proceed without user
    next();
  }
}

// Admin authentication
async function requireAdmin(req, res, next) {
  try {
    if (!req.user) {
      throw new APIError('Authentication required', 401);
    }

    // Check if user has admin role
    const result = await query(
      'SELECT role FROM users WHERE id = $1',
      [req.user.id]
    );

    if (result.rows.length === 0 || result.rows[0].role !== 'admin') {
      logger.warn('Non-admin user attempted admin access', { userId: req.user.id, email: req.user.email });
      throw new APIError('Admin access required', 403);
    }

    req.user.role = 'admin';
    next();
  } catch (error) {
    next(error);
  }
}

// API Key authentication (for proxy requests)
async function authenticateAPIKey(req, res, next) {
  try {
    const apiKey = req.headers['x-api-key'] || req.query.api_key;

    if (!apiKey) {
      throw new APIError('API key required', 401);
    }

    // Check cache first
    let keyData = await getCache(`api_key:${apiKey}`);

    if (!keyData) {
      // Hash the API key and query database
      const crypto = require('crypto');
      const keyHash = crypto.createHash('sha256').update(apiKey).digest('hex');

      const result = await query(`
        SELECT ak.id, ak.user_id, ak.name, ak.permissions, ak.is_active, ak.ip_whitelist,
               u.email, u.first_name, u.last_name, u.status
        FROM api_keys ak
        JOIN users u ON ak.user_id = u.id
        WHERE ak.key_hash = $1 AND ak.is_active = true AND u.status = 'active'
      `, [keyHash]);

      if (result.rows.length === 0) {
        throw new APIError('Invalid API key', 401);
      }

      keyData = result.rows[0];

      // Cache for 5 minutes
      await setCache(`api_key:${apiKey}`, keyData, 300);

      // Update last_used_at
      await query('UPDATE api_keys SET last_used_at = NOW() WHERE id = $1', [keyData.id]);
    }

    // Check IP whitelist if configured
    const ipWhitelist = keyData.ip_whitelist || [];
    if (ipWhitelist.length > 0) {
      const clientIP = req.headers['x-forwarded-for']?.split(',')[0]?.trim() || 
                       req.headers['x-real-ip'] || 
                       req.connection?.remoteAddress ||
                       req.socket?.remoteAddress;
      
      if (!isIPAllowed(clientIP, ipWhitelist)) {
        logger.warn('API request blocked by IP whitelist', { 
          keyId: keyData.id, 
          clientIP, 
          whitelist: ipWhitelist 
        });
        throw new APIError('IP address not in whitelist', 403);
      }
    }

    req.user = {
      id: keyData.user_id,
      email: keyData.email,
      first_name: keyData.first_name,
      last_name: keyData.last_name,
      status: keyData.status
    };
    req.apiKey = {
      id: keyData.id,
      name: keyData.name,
      permissions: keyData.permissions
    };

    next();
  } catch (error) {
    next(error);
  }
}

// Helper function to check if IP is in whitelist (supports CIDR)
function isIPAllowed(clientIP, whitelist) {
  if (!clientIP || whitelist.length === 0) return true;
  
  // Clean up IPv6-mapped IPv4 addresses
  const cleanIP = clientIP.replace(/^::ffff:/, '');
  
  for (const allowed of whitelist) {
    if (allowed.includes('/')) {
      // CIDR notation
      if (isIPInCIDR(cleanIP, allowed)) return true;
    } else {
      // Exact match
      if (cleanIP === allowed) return true;
    }
  }
  return false;
}

// Check if IP is within CIDR range
function isIPInCIDR(ip, cidr) {
  const [range, bits] = cidr.split('/');
  const mask = ~(2 ** (32 - parseInt(bits)) - 1);
  
  const ipNum = ip.split('.').reduce((acc, octet) => (acc << 8) + parseInt(octet), 0);
  const rangeNum = range.split('.').reduce((acc, octet) => (acc << 8) + parseInt(octet), 0);
  
  return (ipNum & mask) === (rangeNum & mask);
}

// Permission check middleware
function requirePermission(permission) {
  return (req, res, next) => {
    if (!req.apiKey || !req.apiKey.permissions.includes(permission)) {
      return next(new APIError('Insufficient permissions', 403));
    }
    next();
  };
}

module.exports = {
  authenticateToken,
  optionalAuth,
  requireAdmin,
  authenticateAPIKey,
  requirePermission
};
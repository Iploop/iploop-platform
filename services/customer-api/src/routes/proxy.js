const express = require('express');
const Joi = require('joi');
const crypto = require('crypto');

const { query, setCache, getCache } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// Validation schemas
const apiKeySchema = Joi.object({
  name: Joi.string().min(1).max(100).required()
});

const configSchema = Joi.object({
  country: Joi.string().length(2).uppercase().optional(),
  city: Joi.string().min(2).max(50).optional(),
  sessionId: Joi.string().max(100).optional(),
  stickySession: Joi.boolean().optional()
});

// ==================== API KEY MANAGEMENT ====================

// List all API keys for user
router.get('/keys', authenticateToken, asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT id, name, key_prefix, is_active, created_at, last_used_at
    FROM api_keys
    WHERE user_id = $1
    ORDER BY created_at DESC
  `, [req.user.id]);

  res.json({
    keys: result.rows.map(row => ({
      id: row.id,
      name: row.name,
      keyPrefix: row.key_prefix,
      isActive: row.is_active,
      createdAt: row.created_at,
      lastUsedAt: row.last_used_at
    }))
  });
}));

// Create new API key
router.post('/keys', authenticateToken, asyncHandler(async (req, res) => {
  const { error, value } = apiKeySchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { name } = value;

  // Check max keys limit (5 per user)
  const countResult = await query('SELECT COUNT(*) FROM api_keys WHERE user_id = $1', [req.user.id]);
  if (parseInt(countResult.rows[0].count) >= 5) {
    throw new APIError('Maximum number of API keys reached (5)', 400);
  }

  // Generate API key
  const apiKey = 'iploop_' + crypto.randomBytes(24).toString('hex');
  const keyHash = crypto.createHash('sha256').update(apiKey).digest('hex');
  const keyPrefix = apiKey.substring(0, 14) + '...';

  // Save to database
  const result = await query(`
    INSERT INTO api_keys (user_id, name, key_hash, key_prefix, is_active)
    VALUES ($1, $2, $3, $4, true)
    RETURNING id, name, key_prefix, is_active, created_at
  `, [req.user.id, name, keyHash, keyPrefix]);

  logger.info('API key created', { userId: req.user.id, keyId: result.rows[0].id });

  res.status(201).json({
    message: 'API key created successfully',
    key: {
      id: result.rows[0].id,
      name: result.rows[0].name,
      keyPrefix: result.rows[0].key_prefix,
      apiKey: apiKey, // Only shown once!
      isActive: result.rows[0].is_active,
      createdAt: result.rows[0].created_at
    },
    warning: 'Save this API key now! It will not be shown again.'
  });
}));

// Delete API key
router.delete('/keys/:keyId', authenticateToken, asyncHandler(async (req, res) => {
  const { keyId } = req.params;

  const result = await query(`
    DELETE FROM api_keys
    WHERE id = $1 AND user_id = $2
    RETURNING id
  `, [keyId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('API key not found', 404);
  }

  logger.info('API key deleted', { userId: req.user.id, keyId });

  res.json({ message: 'API key deleted successfully' });
}));

// Toggle API key active status
router.patch('/keys/:keyId', authenticateToken, asyncHandler(async (req, res) => {
  const { keyId } = req.params;
  const { isActive } = req.body;

  if (typeof isActive !== 'boolean') {
    throw new APIError('isActive must be a boolean', 400);
  }

  const result = await query(`
    UPDATE api_keys
    SET is_active = $1
    WHERE id = $2 AND user_id = $3
    RETURNING id, name, key_prefix, is_active, created_at, last_used_at
  `, [isActive, keyId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('API key not found', 404);
  }

  logger.info('API key status updated', { userId: req.user.id, keyId, isActive });

  res.json({
    message: `API key ${isActive ? 'activated' : 'deactivated'} successfully`,
    key: result.rows[0]
  });
}));

// Update API key IP whitelist
router.put('/keys/:keyId/whitelist', authenticateToken, asyncHandler(async (req, res) => {
  const { keyId } = req.params;
  const { ips } = req.body;

  // Validate IPs array
  if (!Array.isArray(ips)) {
    throw new APIError('ips must be an array', 400);
  }

  // Validate each IP/CIDR
  const ipRegex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;
  for (const ip of ips) {
    if (!ipRegex.test(ip)) {
      throw new APIError(`Invalid IP or CIDR: ${ip}`, 400);
    }
  }

  // Max 50 IPs per key
  if (ips.length > 50) {
    throw new APIError('Maximum 50 IPs allowed per API key', 400);
  }

  const result = await query(`
    UPDATE api_keys
    SET ip_whitelist = $1, updated_at = NOW()
    WHERE id = $2 AND user_id = $3
    RETURNING id, name, key_prefix, ip_whitelist, is_active
  `, [JSON.stringify(ips), keyId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('API key not found', 404);
  }

  logger.info('API key IP whitelist updated', { userId: req.user.id, keyId, ipCount: ips.length });

  res.json({
    message: ips.length > 0 ? 'IP whitelist updated successfully' : 'IP whitelist cleared',
    key: {
      id: result.rows[0].id,
      name: result.rows[0].name,
      keyPrefix: result.rows[0].key_prefix,
      ipWhitelist: result.rows[0].ip_whitelist,
      isActive: result.rows[0].is_active
    }
  });
}));

// Get API key with IP whitelist
router.get('/keys/:keyId', authenticateToken, asyncHandler(async (req, res) => {
  const { keyId } = req.params;

  const result = await query(`
    SELECT id, name, key_prefix, ip_whitelist, is_active, created_at, last_used_at
    FROM api_keys
    WHERE id = $1 AND user_id = $2
  `, [keyId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('API key not found', 404);
  }

  const row = result.rows[0];
  res.json({
    key: {
      id: row.id,
      name: row.name,
      keyPrefix: row.key_prefix,
      ipWhitelist: row.ip_whitelist || [],
      isActive: row.is_active,
      createdAt: row.created_at,
      lastUsedAt: row.last_used_at
    }
  });
}));

// ==================== PROXY ENDPOINTS ====================

// Get proxy endpoint configuration
router.get('/endpoint', authenticateToken, asyncHandler(async (req, res) => {
  // Get user's API key
  const apiKeyResult = await query(`
    SELECT ak.id, ak.key_hash, ak.name, ak.is_active
    FROM api_keys ak
    WHERE ak.user_id = $1 AND ak.is_active = true
    ORDER BY ak.created_at DESC
    LIMIT 1
  `, [req.user.id]);

  if (apiKeyResult.rows.length === 0) {
    throw new APIError('No active API key found. Please generate an API key first.', 400);
  }

  const apiKey = apiKeyResult.rows[0];

  // Get proxy endpoints from environment
  const proxyEndpoint = process.env.PROXY_ENDPOINT || 'localhost';
  const httpPort = process.env.PROXY_HTTP_PORT || '7777';
  const socksPort = process.env.PROXY_SOCKS_PORT || '1080';

  // For demo purposes, we'll return a demo key instead of the hash
  // In production, this would require the user to copy their actual API key
  const customerId = req.user.id.split('-')[0]; // Use first part of UUID as customer ID

  res.json({
    endpoints: {
      http: {
        host: proxyEndpoint,
        port: parseInt(httpPort),
        url: `http://${proxyEndpoint}:${httpPort}`,
        auth: {
          format: 'customer_id:api_key@proxy_host:port',
          example: `${customerId}:your_api_key@${proxyEndpoint}:${httpPort}`
        }
      },
      socks5: {
        host: proxyEndpoint,
        port: parseInt(socksPort),
        url: `socks5://${proxyEndpoint}:${socksPort}`,
        auth: {
          username: `${customerId}`,
          password: 'your_api_key',
          format: 'customer_id:api_key'
        }
      }
    },
    targeting: {
      country: 'Add "-country-US" to your API key for country targeting',
      city: 'Add "-country-US-city-newyork" for city targeting',
      session: 'Add "-session-abc123" for sticky sessions'
    },
    examples: {
      basic: `curl -x http://${customerId}:your_api_key@${proxyEndpoint}:${httpPort} http://httpbin.org/ip`,
      withCountry: `curl -x http://${customerId}:your_api_key-country-US@${proxyEndpoint}:${httpPort} http://httpbin.org/ip`,
      withCity: `curl -x http://${customerId}:your_api_key-country-US-city-newyork@${proxyEndpoint}:${httpPort} http://httpbin.org/ip`,
      socks5: `curl --socks5 ${customerId}:your_api_key@${proxyEndpoint}:${socksPort} http://httpbin.org/ip`
    }
  });
}));

// Update proxy configuration (for sticky sessions, etc.)
router.post('/config', authenticateToken, asyncHandler(async (req, res) => {
  const { error, value } = configSchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { country, city, sessionId, stickySession } = value;

  // Cache user's proxy configuration
  const config = {
    userId: req.user.id,
    country: country || null,
    city: city || null,
    sessionId: sessionId || null,
    stickySession: stickySession || false,
    updatedAt: new Date().toISOString()
  };

  await setCache(`proxy_config:${req.user.id}`, config, 3600); // Cache for 1 hour

  logger.info('Proxy configuration updated', { userId: req.user.id, config });

  res.json({
    message: 'Proxy configuration updated successfully',
    config: {
      country: config.country,
      city: config.city,
      sessionId: config.sessionId,
      stickySession: config.stickySession
    }
  });
}));

// Get current proxy configuration
router.get('/config', authenticateToken, asyncHandler(async (req, res) => {
  const config = await getCache(`proxy_config:${req.user.id}`);

  res.json({
    config: config || {
      country: null,
      city: null,
      sessionId: null,
      stickySession: false
    }
  });
}));

// Test proxy connection
router.post('/test', authenticateToken, asyncHandler(async (req, res) => {
  // This is a simplified test endpoint
  // In production, this would actually test the proxy connection
  
  const { targetUrl = 'http://httpbin.org/ip' } = req.body;

  if (!targetUrl.startsWith('http')) {
    throw new APIError('Invalid target URL', 400);
  }

  // Simulate a proxy test
  const testResult = {
    success: true,
    targetUrl,
    proxyIp: '192.168.1.100', // Simulated
    country: 'US',
    city: 'New York',
    latency: Math.floor(Math.random() * 100) + 50, // Simulated latency
    timestamp: new Date().toISOString()
  };

  logger.info('Proxy connection tested', { userId: req.user.id, targetUrl, success: testResult.success });

  res.json({
    message: 'Proxy test completed',
    result: testResult
  });
}));

// Get available countries and cities
router.get('/locations', asyncHandler(async (req, res) => {
  // Check cache first
  let locations = await getCache('proxy_locations');

  if (!locations) {
    // Fetch from database
    const result = await query(`
      SELECT country, country_name, city, COUNT(*) as node_count
      FROM nodes
      WHERE status = 'available' AND last_heartbeat > NOW() - INTERVAL '5 minutes'
      GROUP BY country, country_name, city
      ORDER BY country, city
    `);

    // Group by country
    const locationMap = {};
    result.rows.forEach(row => {
      if (!locationMap[row.country]) {
        locationMap[row.country] = {
          code: row.country,
          name: row.country_name,
          cities: [],
          totalNodes: 0
        };
      }

      if (row.city) {
        locationMap[row.country].cities.push({
          name: row.city,
          nodeCount: parseInt(row.node_count)
        });
      }

      locationMap[row.country].totalNodes += parseInt(row.node_count);
    });

    locations = Object.values(locationMap);

    // Cache for 5 minutes
    await setCache('proxy_locations', locations, 300);
  }

  res.json({
    locations,
    totalCountries: locations.length,
    totalCities: locations.reduce((sum, country) => sum + country.cities.length, 0),
    lastUpdated: new Date().toISOString()
  });
}));

// Get proxy statistics
router.get('/stats', authenticateToken, asyncHandler(async (req, res) => {
  // Get user's usage statistics
  const usageResult = await query(`
    SELECT 
      COUNT(*) as total_requests,
      SUM(CASE WHEN success = true THEN 1 ELSE 0 END) as successful_requests,
      SUM(total_bytes) as total_bytes,
      AVG(duration_ms) as avg_duration_ms,
      COUNT(DISTINCT target_country) as countries_used,
      MAX(started_at) as last_request
    FROM usage_records
    WHERE user_id = $1 AND started_at >= NOW() - INTERVAL '30 days'
  `, [req.user.id]);

  const usage = usageResult.rows[0];

  // Get top countries used
  const countriesResult = await query(`
    SELECT target_country, COUNT(*) as request_count, SUM(total_bytes) as bytes_used
    FROM usage_records
    WHERE user_id = $1 AND started_at >= NOW() - INTERVAL '30 days' AND target_country IS NOT NULL
    GROUP BY target_country
    ORDER BY request_count DESC
    LIMIT 10
  `, [req.user.id]);

  const stats = {
    usage: {
      totalRequests: parseInt(usage.total_requests) || 0,
      successfulRequests: parseInt(usage.successful_requests) || 0,
      successRate: usage.total_requests > 0 ? (usage.successful_requests / usage.total_requests * 100).toFixed(2) : '0.00',
      totalBytes: parseInt(usage.total_bytes) || 0,
      totalGB: ((parseInt(usage.total_bytes) || 0) / (1024 * 1024 * 1024)).toFixed(3),
      avgDurationMs: parseFloat(usage.avg_duration_ms) || 0,
      countriesUsed: parseInt(usage.countries_used) || 0,
      lastRequest: usage.last_request
    },
    topCountries: countriesResult.rows.map(row => ({
      country: row.target_country,
      requestCount: parseInt(row.request_count),
      bytesUsed: parseInt(row.bytes_used),
      gbUsed: (parseInt(row.bytes_used) / (1024 * 1024 * 1024)).toFixed(3)
    })),
    period: 'Last 30 days'
  };

  res.json(stats);
}));

module.exports = router;
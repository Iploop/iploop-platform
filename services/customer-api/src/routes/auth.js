const express = require('express');
const bcrypt = require('bcryptjs');
const jwt = require('jsonwebtoken');
const Joi = require('joi');
const crypto = require('crypto');
const { v4: uuidv4 } = require('uuid');

const { query, transaction, setCache, delCache } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// Validation schemas
const registerSchema = Joi.object({
  email: Joi.string().email().required(),
  password: Joi.string().min(8).required(),
  firstName: Joi.string().min(2).max(50).required(),
  lastName: Joi.string().min(2).max(50).required(),
  company: Joi.string().max(100).optional()
});

const loginSchema = Joi.object({
  email: Joi.string().email().required(),
  password: Joi.string().required()
});

const changePasswordSchema = Joi.object({
  currentPassword: Joi.string().required(),
  newPassword: Joi.string().min(8).required()
});

// Register new user
router.post('/register', asyncHandler(async (req, res) => {
  const { error, value } = registerSchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { email, password, firstName, lastName, company } = value;

  // Check if user already exists
  const existingUser = await query('SELECT id FROM users WHERE email = $1', [email]);
  if (existingUser.rows.length > 0) {
    throw new APIError('User already exists with this email', 409);
  }

  // Hash password
  const saltRounds = 12;
  const passwordHash = await bcrypt.hash(password, saltRounds);

  // Create user and assign starter plan
  const result = await transaction(async (client) => {
    // Create user
    const userResult = await client.query(`
      INSERT INTO users (email, password_hash, first_name, last_name, company, status, email_verified)
      VALUES ($1, $2, $3, $4, $5, 'active', true)
      RETURNING id, email, first_name, last_name, company, created_at
    `, [email, passwordHash, firstName, lastName, company || null]);

    const user = userResult.rows[0];

    // Get starter plan
    const planResult = await client.query('SELECT id FROM plans WHERE name = $1', ['Starter']);
    if (planResult.rows.length === 0) {
      throw new APIError('Default plan not found', 500);
    }

    // Assign starter plan with 5GB free
    await client.query(`
      INSERT INTO user_plans (user_id, plan_id, gb_balance, status)
      VALUES ($1, $2, 5.0, 'active')
    `, [user.id, planResult.rows[0].id]);

    return user;
  });

  // Generate JWT token
  const token = jwt.sign(
    { userId: result.id, email: result.email },
    process.env.JWT_SECRET,
    { expiresIn: process.env.JWT_EXPIRES_IN || '24h' }
  );

  logger.info('User registered successfully', { userId: result.id, email: result.email });

  res.status(201).json({
    message: 'User registered successfully',
    user: {
      id: result.id,
      email: result.email,
      firstName: result.first_name,
      lastName: result.last_name,
      company: result.company,
      createdAt: result.created_at
    },
    token
  });
}));

// Login user
router.post('/login', asyncHandler(async (req, res) => {
  const { error, value } = loginSchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { email, password } = value;

  // Get user
  const result = await query(`
    SELECT id, email, password_hash, first_name, last_name, company, status, role
    FROM users
    WHERE email = $1
  `, [email]);

  if (result.rows.length === 0) {
    throw new APIError('Invalid email or password', 401);
  }

  const user = result.rows[0];

  if (user.status !== 'active') {
    throw new APIError('Account is not active', 401);
  }

  // Verify password
  const isValidPassword = await bcrypt.compare(password, user.password_hash);
  if (!isValidPassword) {
    throw new APIError('Invalid email or password', 401);
  }

  // Update last login
  await query('UPDATE users SET last_login_at = NOW() WHERE id = $1', [user.id]);

  // Generate JWT token
  const token = jwt.sign(
    { userId: user.id, email: user.email },
    process.env.JWT_SECRET,
    { expiresIn: process.env.JWT_EXPIRES_IN || '24h' }
  );

  // Cache user data
  await setCache(`user:${user.id}`, user, 900);

  logger.info('User logged in successfully', { userId: user.id, email: user.email });

  res.json({
    message: 'Login successful',
    user: {
      id: user.id,
      email: user.email,
      firstName: user.first_name,
      lastName: user.last_name,
      company: user.company,
      role: user.role || 'customer'
    },
    token
  });
}));

// Get current user profile
router.get('/profile', authenticateToken, asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT u.id, u.email, u.first_name, u.last_name, u.company, u.created_at, u.last_login_at,
           up.gb_balance, up.gb_used, p.name as plan_name
    FROM users u
    LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
    LEFT JOIN plans p ON up.plan_id = p.id
    WHERE u.id = $1
  `, [req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('User not found', 404);
  }

  const user = result.rows[0];

  res.json({
    user: {
      id: user.id,
      email: user.email,
      firstName: user.first_name,
      lastName: user.last_name,
      company: user.company,
      createdAt: user.created_at,
      lastLoginAt: user.last_login_at,
      plan: user.plan_name,
      gbBalance: parseFloat(user.gb_balance) || 0,
      gbUsed: parseFloat(user.gb_used) || 0
    }
  });
}));

// Generate API key
router.post('/api-keys', authenticateToken, asyncHandler(async (req, res) => {
  const { name, permissions = ['proxy'] } = req.body;

  if (!name || name.length < 3) {
    throw new APIError('API key name is required (min 3 characters)', 400);
  }

  // Generate API key
  const apiKey = crypto.randomBytes(32).toString('hex');
  const keyHash = crypto.createHash('sha256').update(apiKey).digest('hex');

  const result = await query(`
    INSERT INTO api_keys (user_id, key_hash, name, permissions)
    VALUES ($1, $2, $3, $4)
    RETURNING id, name, permissions, created_at
  `, [req.user.id, keyHash, name, JSON.stringify(permissions)]);

  logger.info('API key generated', { userId: req.user.id, keyName: name });

  res.status(201).json({
    message: 'API key generated successfully',
    apiKey: {
      id: result.rows[0].id,
      key: apiKey, // Only returned once!
      name: result.rows[0].name,
      permissions: result.rows[0].permissions,
      createdAt: result.rows[0].created_at
    }
  });
}));

// List API keys
router.get('/api-keys', authenticateToken, asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT id, name, permissions, is_active, created_at, last_used_at
    FROM api_keys
    WHERE user_id = $1
    ORDER BY created_at DESC
  `, [req.user.id]);

  res.json({
    apiKeys: result.rows.map(key => ({
      id: key.id,
      name: key.name,
      permissions: key.permissions,
      isActive: key.is_active,
      createdAt: key.created_at,
      lastUsedAt: key.last_used_at
    }))
  });
}));

// Revoke API key
router.delete('/api-keys/:keyId', authenticateToken, asyncHandler(async (req, res) => {
  const { keyId } = req.params;

  const result = await query(`
    UPDATE api_keys
    SET is_active = false, updated_at = NOW()
    WHERE id = $1 AND user_id = $2
    RETURNING name
  `, [keyId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('API key not found', 404);
  }

  // Clear cache
  await delCache(`api_key:*`);

  logger.info('API key revoked', { userId: req.user.id, keyId, keyName: result.rows[0].name });

  res.json({
    message: 'API key revoked successfully'
  });
}));

// Change password
router.put('/password', authenticateToken, asyncHandler(async (req, res) => {
  const { error, value } = changePasswordSchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { currentPassword, newPassword } = value;

  // Get current password hash
  const result = await query('SELECT password_hash FROM users WHERE id = $1', [req.user.id]);
  if (result.rows.length === 0) {
    throw new APIError('User not found', 404);
  }

  // Verify current password
  const isValidPassword = await bcrypt.compare(currentPassword, result.rows[0].password_hash);
  if (!isValidPassword) {
    throw new APIError('Current password is incorrect', 400);
  }

  // Hash new password
  const saltRounds = 12;
  const newPasswordHash = await bcrypt.hash(newPassword, saltRounds);

  // Update password
  await query('UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2', 
    [newPasswordHash, req.user.id]);

  logger.info('Password changed', { userId: req.user.id });

  res.json({
    message: 'Password changed successfully'
  });
}));

// Logout (blacklist token)
router.post('/logout', authenticateToken, asyncHandler(async (req, res) => {
  // Blacklist the current token
  const decoded = jwt.decode(req.token);
  const ttl = decoded.exp - Math.floor(Date.now() / 1000);
  
  if (ttl > 0) {
    await setCache(`blacklist:${req.token}`, true, ttl);
  }

  // Clear user cache
  await delCache(`user:${req.user.id}`);

  logger.info('User logged out', { userId: req.user.id });

  res.json({
    message: 'Logged out successfully'
  });
}));

module.exports = router;
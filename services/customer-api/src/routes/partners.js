const express = require('express');
const Joi = require('joi');
const crypto = require('crypto');

const { query } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken, requireAdmin } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// Validation schemas
const partnerSchema = Joi.object({
  name: Joi.string().min(2).max(100).required(),
  email: Joi.string().email().required(),
  revenueShare: Joi.number().min(0).max(100).default(70)
});

// List all partners (admin only)
router.get('/', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT p.id, p.name, p.email, p.api_key_prefix, p.is_active, p.revenue_share,
           p.total_nodes, p.total_earnings, p.created_at,
           COUNT(n.id) as active_nodes
    FROM partners p
    LEFT JOIN nodes n ON n.partner_id = p.id AND n.status = 'available'
    GROUP BY p.id
    ORDER BY p.created_at DESC
  `);

  res.json({
    partners: result.rows.map(row => ({
      id: row.id,
      name: row.name,
      email: row.email,
      apiKeyPrefix: row.api_key_prefix,
      isActive: row.is_active,
      revenueShare: parseFloat(row.revenue_share),
      totalNodes: row.total_nodes,
      activeNodes: parseInt(row.active_nodes),
      totalEarnings: parseFloat(row.total_earnings),
      createdAt: row.created_at
    }))
  });
}));

// Create new partner (admin only)
router.post('/', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const { error, value } = partnerSchema.validate(req.body);
  if (error) {
    throw new APIError(error.details[0].message, 400);
  }

  const { name, email, revenueShare } = value;

  // Check if partner already exists
  const existing = await query('SELECT id FROM partners WHERE email = $1', [email]);
  if (existing.rows.length > 0) {
    throw new APIError('Partner with this email already exists', 400);
  }

  // Generate API key
  const apiKey = 'iplp_' + crypto.randomBytes(24).toString('hex');
  const keyHash = crypto.createHash('sha256').update(apiKey).digest('hex');
  const keyPrefix = apiKey.substring(0, 12) + '...';

  const result = await query(`
    INSERT INTO partners (name, email, api_key_hash, api_key_prefix, revenue_share)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, name, email, api_key_prefix, is_active, revenue_share, created_at
  `, [name, email, keyHash, keyPrefix, revenueShare]);

  logger.info('Partner created', { partnerId: result.rows[0].id, name });

  res.status(201).json({
    message: 'Partner created successfully',
    partner: {
      id: result.rows[0].id,
      name: result.rows[0].name,
      email: result.rows[0].email,
      apiKey: apiKey, // Only shown once!
      apiKeyPrefix: result.rows[0].api_key_prefix,
      isActive: result.rows[0].is_active,
      revenueShare: parseFloat(result.rows[0].revenue_share),
      createdAt: result.rows[0].created_at
    },
    warning: 'Save this API key now! It will not be shown again. Share it with the partner for SDK integration.'
  });
}));

// Get partner details (admin only)
router.get('/:partnerId', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const { partnerId } = req.params;

  const result = await query(`
    SELECT p.*, COUNT(n.id) as active_nodes
    FROM partners p
    LEFT JOIN nodes n ON n.partner_id = p.id AND n.status = 'available'
    WHERE p.id = $1
    GROUP BY p.id
  `, [partnerId]);

  if (result.rows.length === 0) {
    throw new APIError('Partner not found', 404);
  }

  const row = result.rows[0];

  // Get partner's nodes
  const nodesResult = await query(`
    SELECT id, device_id, country, city, status, last_heartbeat
    FROM nodes
    WHERE partner_id = $1
    ORDER BY last_heartbeat DESC
    LIMIT 100
  `, [partnerId]);

  res.json({
    partner: {
      id: row.id,
      name: row.name,
      email: row.email,
      apiKeyPrefix: row.api_key_prefix,
      isActive: row.is_active,
      revenueShare: parseFloat(row.revenue_share),
      totalNodes: row.total_nodes,
      activeNodes: parseInt(row.active_nodes),
      totalEarnings: parseFloat(row.total_earnings),
      createdAt: row.created_at,
      updatedAt: row.updated_at
    },
    nodes: nodesResult.rows
  });
}));

// Update partner (admin only)
router.patch('/:partnerId', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const { partnerId } = req.params;
  const { isActive, revenueShare, name } = req.body;

  const updates = [];
  const values = [];
  let paramIndex = 1;

  if (typeof isActive === 'boolean') {
    updates.push(`is_active = $${paramIndex++}`);
    values.push(isActive);
  }
  if (typeof revenueShare === 'number') {
    updates.push(`revenue_share = $${paramIndex++}`);
    values.push(revenueShare);
  }
  if (name) {
    updates.push(`name = $${paramIndex++}`);
    values.push(name);
  }

  if (updates.length === 0) {
    throw new APIError('No valid fields to update', 400);
  }

  updates.push(`updated_at = NOW()`);
  values.push(partnerId);

  const result = await query(`
    UPDATE partners
    SET ${updates.join(', ')}
    WHERE id = $${paramIndex}
    RETURNING id, name, email, is_active, revenue_share, updated_at
  `, values);

  if (result.rows.length === 0) {
    throw new APIError('Partner not found', 404);
  }

  logger.info('Partner updated', { partnerId, updates: req.body });

  res.json({
    message: 'Partner updated successfully',
    partner: result.rows[0]
  });
}));

// Regenerate partner API key (admin only)
router.post('/:partnerId/regenerate-key', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const { partnerId } = req.params;

  // Generate new API key
  const apiKey = 'iplp_' + crypto.randomBytes(24).toString('hex');
  const keyHash = crypto.createHash('sha256').update(apiKey).digest('hex');
  const keyPrefix = apiKey.substring(0, 12) + '...';

  const result = await query(`
    UPDATE partners
    SET api_key_hash = $1, api_key_prefix = $2, updated_at = NOW()
    WHERE id = $3
    RETURNING id, name, api_key_prefix
  `, [keyHash, keyPrefix, partnerId]);

  if (result.rows.length === 0) {
    throw new APIError('Partner not found', 404);
  }

  logger.info('Partner API key regenerated', { partnerId });

  res.json({
    message: 'API key regenerated successfully',
    partner: {
      id: result.rows[0].id,
      name: result.rows[0].name,
      apiKey: apiKey, // Only shown once!
      apiKeyPrefix: result.rows[0].api_key_prefix
    },
    warning: 'Save this API key now! The old key is no longer valid.'
  });
}));

// Delete partner (admin only)
router.delete('/:partnerId', authenticateToken, requireAdmin, asyncHandler(async (req, res) => {
  const { partnerId } = req.params;

  // Check if partner has active nodes
  const nodesCheck = await query(`
    SELECT COUNT(*) FROM nodes WHERE partner_id = $1 AND status = 'available'
  `, [partnerId]);

  if (parseInt(nodesCheck.rows[0].count) > 0) {
    throw new APIError('Cannot delete partner with active nodes. Deactivate first.', 400);
  }

  const result = await query(`
    DELETE FROM partners WHERE id = $1 RETURNING id, name
  `, [partnerId]);

  if (result.rows.length === 0) {
    throw new APIError('Partner not found', 404);
  }

  logger.info('Partner deleted', { partnerId, name: result.rows[0].name });

  res.json({ message: 'Partner deleted successfully' });
}));

module.exports = router;

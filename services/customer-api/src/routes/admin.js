const express = require('express');
const bcrypt = require('bcryptjs');
const { query, transaction } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken, requireAdmin } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// All admin routes require authentication + admin role
router.use(authenticateToken);
router.use(requireAdmin);

// Get all users
router.get('/users', asyncHandler(async (req, res) => {
  const { page = 1, limit = 50, status, search } = req.query;
  const offset = (page - 1) * limit;

  let whereClause = '1=1';
  const params = [];
  let paramCount = 0;

  if (status) {
    paramCount++;
    whereClause += ` AND u.status = $${paramCount}`;
    params.push(status);
  }

  if (search) {
    paramCount++;
    whereClause += ` AND (u.email ILIKE $${paramCount} OR u.first_name ILIKE $${paramCount} OR u.last_name ILIKE $${paramCount})`;
    params.push(`%${search}%`);
  }

  const result = await query(`
    SELECT 
      u.id, u.email, u.first_name, u.last_name, u.company, 
      u.status, u.role, u.created_at, u.last_login_at,
      COALESCE(up.gb_balance, 0) as gb_balance,
      COALESCE(up.gb_used, 0) as gb_used,
      COALESCE(p.name, 'Free') as plan_name
    FROM users u
    LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
    LEFT JOIN plans p ON up.plan_id = p.id
    WHERE ${whereClause}
    ORDER BY u.created_at DESC
    LIMIT $${paramCount + 1} OFFSET $${paramCount + 2}
  `, [...params, limit, offset]);

  const countResult = await query(`
    SELECT COUNT(*) FROM users u WHERE ${whereClause}
  `, params);

  res.json({
    users: result.rows.map(row => ({
      id: row.id,
      email: row.email,
      firstName: row.first_name,
      lastName: row.last_name,
      company: row.company,
      status: row.status,
      role: row.role,
      planName: row.plan_name,
      gbBalance: parseFloat(row.gb_balance),
      gbUsed: parseFloat(row.gb_used),
      createdAt: row.created_at,
      lastLoginAt: row.last_login_at
    })),
    total: parseInt(countResult.rows[0].count),
    page: parseInt(page),
    limit: parseInt(limit)
  });
}));

// Create new user
router.post('/users/create', asyncHandler(async (req, res) => {
  const { email, password, firstName, lastName, company, role = 'customer' } = req.body;

  if (!email || !password || !firstName || !lastName) {
    throw new APIError('Email, password, firstName and lastName are required', 400);
  }

  if (password.length < 8) {
    throw new APIError('Password must be at least 8 characters', 400);
  }

  // Check if user already exists
  const existingUser = await query('SELECT id FROM users WHERE email = $1', [email]);
  if (existingUser.rows.length > 0) {
    throw new APIError('User already exists with this email', 409);
  }

  // Hash password
  const saltRounds = 12;
  const passwordHash = await bcrypt.hash(password, saltRounds);

  // Create user
  const result = await transaction(async (client) => {
    const userResult = await client.query(`
      INSERT INTO users (email, password_hash, first_name, last_name, company, status, role, email_verified)
      VALUES ($1, $2, $3, $4, $5, 'active', $6, true)
      RETURNING id, email, first_name, last_name, company, status, role, created_at
    `, [email, passwordHash, firstName, lastName, company || null, role]);

    const user = userResult.rows[0];

    // Get starter plan
    const planResult = await client.query('SELECT id FROM plans WHERE name = $1', ['Starter']);
    if (planResult.rows.length > 0) {
      await client.query(`
        INSERT INTO user_plans (user_id, plan_id, gb_balance, status)
        VALUES ($1, $2, 5.0, 'active')
      `, [user.id, planResult.rows[0].id]);
    }

    return user;
  });

  logger.info('User created by admin', { adminId: req.user.id, newUserId: result.id, email: result.email });

  res.status(201).json({
    message: 'User created successfully',
    user: {
      id: result.id,
      email: result.email,
      firstName: result.first_name,
      lastName: result.last_name,
      company: result.company,
      status: result.status,
      role: result.role,
      createdAt: result.created_at
    }
  });
}));

// Get single user
router.get('/users/:userId', asyncHandler(async (req, res) => {
  const { userId } = req.params;

  const result = await query(`
    SELECT 
      u.*, 
      COALESCE(up.gb_balance, 0) as gb_balance,
      COALESCE(up.gb_used, 0) as gb_used,
      p.name as plan_name,
      p.id as plan_id
    FROM users u
    LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
    LEFT JOIN plans p ON up.plan_id = p.id
    WHERE u.id = $1
  `, [userId]);

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
      status: user.status,
      role: user.role,
      planId: user.plan_id,
      planName: user.plan_name,
      gbBalance: parseFloat(user.gb_balance),
      gbUsed: parseFloat(user.gb_used),
      createdAt: user.created_at,
      lastLoginAt: user.last_login_at
    }
  });
}));

// Update user
router.put('/users/:userId', asyncHandler(async (req, res) => {
  const { userId } = req.params;
  const { status, role, gbBalance, planId } = req.body;

  await transaction(async (client) => {
    // Update user
    if (status || role) {
      const updates = [];
      const values = [];
      let idx = 1;

      if (status) {
        updates.push(`status = $${idx++}`);
        values.push(status);
      }
      if (role) {
        updates.push(`role = $${idx++}`);
        values.push(role);
      }

      values.push(userId);
      await client.query(`
        UPDATE users SET ${updates.join(', ')}, updated_at = NOW() 
        WHERE id = $${idx}
      `, values);
    }

    // Update plan/balance
    if (gbBalance !== undefined || planId) {
      const existing = await client.query(
        'SELECT id FROM user_plans WHERE user_id = $1 AND status = \'active\'',
        [userId]
      );

      if (existing.rows.length > 0) {
        const updates = [];
        const values = [];
        let idx = 1;

        if (gbBalance !== undefined) {
          updates.push(`gb_balance = $${idx++}`);
          values.push(gbBalance);
        }
        if (planId) {
          updates.push(`plan_id = $${idx++}`);
          values.push(planId);
        }

        values.push(existing.rows[0].id);
        await client.query(`
          UPDATE user_plans SET ${updates.join(', ')}, updated_at = NOW()
          WHERE id = $${idx}
        `, values);
      } else if (planId) {
        await client.query(`
          INSERT INTO user_plans (user_id, plan_id, gb_balance, status)
          VALUES ($1, $2, $3, 'active')
        `, [userId, planId, gbBalance || 0]);
      }
    }
  });

  logger.info('User updated by admin', { adminId: req.user.id, targetUserId: userId });

  res.json({ message: 'User updated successfully' });
}));

// Get all plans
router.get('/plans', asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT id, name, description, price_per_gb, included_gb, max_concurrent_connections, features, is_active, created_at
    FROM plans
    ORDER BY price_per_gb ASC
  `);

  res.json({
    plans: result.rows.map(row => ({
      id: row.id,
      name: row.name,
      description: row.description,
      monthlyGb: parseFloat(row.included_gb) || 0,
      pricePerGb: parseFloat(row.price_per_gb),
      maxConnections: row.max_concurrent_connections,
      features: row.features,
      isActive: row.is_active,
      createdAt: row.created_at
    }))
  });
}));

// Create plan
router.post('/plans', asyncHandler(async (req, res) => {
  const { name, description, pricePerGb, includedGb, maxConnections } = req.body;

  if (!name || pricePerGb === undefined) {
    throw new APIError('Name and pricePerGb are required', 400);
  }

  const result = await query(`
    INSERT INTO plans (name, description, price_per_gb, included_gb, max_concurrent_connections, features, is_active)
    VALUES ($1, $2, $3, $4, $5, $6, true)
    RETURNING id, name, description, price_per_gb, included_gb, max_concurrent_connections, features, is_active
  `, [name, description || '', pricePerGb, includedGb || 0, maxConnections || 10, {}]);

  logger.info('Plan created by admin', { adminId: req.user.id, planName: name });

  res.status(201).json({
    message: 'Plan created successfully',
    plan: {
      id: result.rows[0].id,
      name: result.rows[0].name,
      description: result.rows[0].description,
      pricePerGb: parseFloat(result.rows[0].price_per_gb),
      monthlyGb: parseFloat(result.rows[0].included_gb),
      maxConnections: result.rows[0].max_concurrent_connections,
      isActive: result.rows[0].is_active
    }
  });
}));

// Update plan
router.put('/plans/:planId', asyncHandler(async (req, res) => {
  const { planId } = req.params;
  const { name, monthlyGb, pricePerGb, features, isActive } = req.body;

  const updates = [];
  const values = [];
  let idx = 1;

  if (name) { updates.push(`name = $${idx++}`); values.push(name); }
  if (monthlyGb !== undefined) { updates.push(`monthly_gb = $${idx++}`); values.push(monthlyGb); }
  if (pricePerGb !== undefined) { updates.push(`price_per_gb = $${idx++}`); values.push(pricePerGb); }
  if (features) { updates.push(`features = $${idx++}`); values.push(features); }
  if (isActive !== undefined) { updates.push(`is_active = $${idx++}`); values.push(isActive); }

  if (updates.length === 0) {
    throw new APIError('No fields to update', 400);
  }

  values.push(planId);
  await query(`
    UPDATE plans SET ${updates.join(', ')}, updated_at = NOW()
    WHERE id = $${idx}
  `, values);

  logger.info('Plan updated by admin', { adminId: req.user.id, planId });

  res.json({ message: 'Plan updated successfully' });
}));

// Get system stats
router.get('/stats', asyncHandler(async (req, res) => {
  const [usersResult, plansResult, usageResult] = await Promise.all([
    query(`
      SELECT 
        COUNT(*) as total,
        COUNT(*) FILTER (WHERE status = 'active') as active,
        COUNT(*) FILTER (WHERE role = 'admin') as admins
      FROM users
    `),
    query('SELECT COUNT(*) as total FROM plans WHERE is_active = true'),
    query(`
      SELECT 
        COUNT(*) as total_requests,
        COALESCE(SUM(bytes_transferred), 0) as total_bytes
      FROM usage_logs
      WHERE created_at >= NOW() - INTERVAL '30 days'
    `)
  ]);

  res.json({
    users: {
      total: parseInt(usersResult.rows[0].total),
      active: parseInt(usersResult.rows[0].active),
      admins: parseInt(usersResult.rows[0].admins)
    },
    plans: {
      active: parseInt(plansResult.rows[0].total)
    },
    usage: {
      totalRequests: parseInt(usageResult.rows[0].total_requests),
      totalGb: (parseInt(usageResult.rows[0].total_bytes) / (1024 * 1024 * 1024)).toFixed(2)
    }
  });
}));

// Delete user
router.delete('/users/:userId', asyncHandler(async (req, res) => {
  const { userId } = req.params;

  // Prevent self-deletion
  if (userId === req.user.id) {
    throw new APIError('Cannot delete your own account', 400);
  }

  const result = await query('DELETE FROM users WHERE id = $1 RETURNING email', [userId]);
  
  if (result.rows.length === 0) {
    throw new APIError('User not found', 404);
  }

  logger.info('User deleted by admin', { adminId: req.user.id, deletedUserId: userId, email: result.rows[0].email });

  res.json({ message: 'User deleted successfully' });
}));

// Make user admin
router.post('/users/:userId/make-admin', asyncHandler(async (req, res) => {
  const { userId } = req.params;

  await query('UPDATE users SET role = \'admin\' WHERE id = $1', [userId]);
  
  logger.info('User promoted to admin', { adminId: req.user.id, targetUserId: userId });

  res.json({ message: 'User promoted to admin successfully' });
}));

// Remove admin
router.post('/users/:userId/remove-admin', asyncHandler(async (req, res) => {
  const { userId } = req.params;

  // Prevent removing own admin
  if (userId === req.user.id) {
    throw new APIError('Cannot remove your own admin role', 400);
  }

  await query('UPDATE users SET role = \'customer\' WHERE id = $1', [userId]);
  
  logger.info('Admin role removed from user', { adminId: req.user.id, targetUserId: userId });

  res.json({ message: 'Admin role removed successfully' });
}));

module.exports = router;

const express = require('express');
const crypto = require('crypto');
const { query, transaction } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// All routes require authentication
router.use(authenticateToken);

// List webhooks
router.get('/', asyncHandler(async (req, res) => {
  const result = await query(`
    SELECT id, url, events, is_active, secret_preview, created_at, last_triggered_at, failure_count
    FROM webhooks
    WHERE user_id = $1
    ORDER BY created_at DESC
  `, [req.user.id]);

  res.json({
    webhooks: result.rows.map(row => ({
      id: row.id,
      url: row.url,
      events: row.events,
      isActive: row.is_active,
      secretPreview: row.secret_preview,
      createdAt: row.created_at,
      lastTriggeredAt: row.last_triggered_at,
      failureCount: row.failure_count
    }))
  });
}));

// Create webhook
router.post('/', asyncHandler(async (req, res) => {
  const { url, events } = req.body;

  if (!url || !events || events.length === 0) {
    throw new APIError('URL and at least one event are required', 400);
  }

  // Validate URL
  try {
    new URL(url);
  } catch {
    throw new APIError('Invalid URL', 400);
  }

  // Validate events
  const validEvents = ['usage.threshold', 'balance.low', 'node.connected', 'node.disconnected', 'request.failed'];
  const invalidEvents = events.filter(e => !validEvents.includes(e));
  if (invalidEvents.length > 0) {
    throw new APIError(`Invalid events: ${invalidEvents.join(', ')}. Valid: ${validEvents.join(', ')}`, 400);
  }

  // Generate secret
  const secret = crypto.randomBytes(32).toString('hex');
  const secretPreview = secret.substring(0, 8) + '...' + secret.substring(secret.length - 4);

  const result = await query(`
    INSERT INTO webhooks (user_id, url, events, secret, secret_preview, is_active)
    VALUES ($1, $2, $3, $4, $5, true)
    RETURNING id, url, events, is_active, secret_preview, created_at
  `, [req.user.id, url, JSON.stringify(events), secret, secretPreview]);

  logger.info('Webhook created', { userId: req.user.id, webhookId: result.rows[0].id });

  res.status(201).json({
    message: 'Webhook created successfully',
    webhook: {
      id: result.rows[0].id,
      url: result.rows[0].url,
      events: result.rows[0].events,
      isActive: result.rows[0].is_active,
      secret: secret, // Only returned once!
      createdAt: result.rows[0].created_at
    }
  });
}));

// Update webhook
router.put('/:webhookId', asyncHandler(async (req, res) => {
  const { webhookId } = req.params;
  const { url, events, isActive } = req.body;

  const updates = [];
  const values = [];
  let idx = 1;

  if (url) {
    try { new URL(url); } catch { throw new APIError('Invalid URL', 400); }
    updates.push(`url = $${idx++}`);
    values.push(url);
  }
  if (events) {
    updates.push(`events = $${idx++}`);
    values.push(JSON.stringify(events));
  }
  if (isActive !== undefined) {
    updates.push(`is_active = $${idx++}`);
    values.push(isActive);
  }

  if (updates.length === 0) {
    throw new APIError('No fields to update', 400);
  }

  values.push(webhookId, req.user.id);
  const result = await query(`
    UPDATE webhooks 
    SET ${updates.join(', ')}, updated_at = NOW()
    WHERE id = $${idx++} AND user_id = $${idx}
    RETURNING id
  `, values);

  if (result.rows.length === 0) {
    throw new APIError('Webhook not found', 404);
  }

  res.json({ message: 'Webhook updated successfully' });
}));

// Delete webhook
router.delete('/:webhookId', asyncHandler(async (req, res) => {
  const { webhookId } = req.params;

  const result = await query(
    'DELETE FROM webhooks WHERE id = $1 AND user_id = $2 RETURNING id',
    [webhookId, req.user.id]
  );

  if (result.rows.length === 0) {
    throw new APIError('Webhook not found', 404);
  }

  logger.info('Webhook deleted', { userId: req.user.id, webhookId });
  res.json({ message: 'Webhook deleted successfully' });
}));

// Test webhook
router.post('/:webhookId/test', asyncHandler(async (req, res) => {
  const { webhookId } = req.params;

  const result = await query(
    'SELECT url, secret FROM webhooks WHERE id = $1 AND user_id = $2',
    [webhookId, req.user.id]
  );

  if (result.rows.length === 0) {
    throw new APIError('Webhook not found', 404);
  }

  const { url, secret } = result.rows[0];

  // Send test event
  const payload = {
    event: 'test',
    timestamp: new Date().toISOString(),
    data: {
      message: 'This is a test webhook from IPLoop',
      userId: req.user.id
    }
  };

  const signature = crypto
    .createHmac('sha256', secret)
    .update(JSON.stringify(payload))
    .digest('hex');

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-IPLoop-Signature': signature,
        'X-IPLoop-Event': 'test'
      },
      body: JSON.stringify(payload),
      timeout: 10000
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }

    await query(
      'UPDATE webhooks SET last_triggered_at = NOW(), failure_count = 0 WHERE id = $1',
      [webhookId]
    );

    res.json({ 
      message: 'Test webhook sent successfully',
      statusCode: response.status 
    });
  } catch (error) {
    await query(
      'UPDATE webhooks SET failure_count = failure_count + 1 WHERE id = $1',
      [webhookId]
    );

    throw new APIError(`Webhook test failed: ${error.message}`, 502);
  }
}));

// Regenerate secret
router.post('/:webhookId/regenerate-secret', asyncHandler(async (req, res) => {
  const { webhookId } = req.params;

  const newSecret = crypto.randomBytes(32).toString('hex');
  const secretPreview = newSecret.substring(0, 8) + '...' + newSecret.substring(newSecret.length - 4);

  const result = await query(`
    UPDATE webhooks 
    SET secret = $1, secret_preview = $2, updated_at = NOW()
    WHERE id = $3 AND user_id = $4
    RETURNING id
  `, [newSecret, secretPreview, webhookId, req.user.id]);

  if (result.rows.length === 0) {
    throw new APIError('Webhook not found', 404);
  }

  logger.info('Webhook secret regenerated', { userId: req.user.id, webhookId });

  res.json({
    message: 'Secret regenerated successfully',
    secret: newSecret // Only returned once!
  });
}));

module.exports = router;

// Helper function to trigger webhooks (called from other services)
module.exports.triggerWebhooks = async function(userId, event, data) {
  try {
    const result = await query(`
      SELECT id, url, secret FROM webhooks 
      WHERE user_id = $1 AND is_active = true AND events @> $2
    `, [userId, JSON.stringify([event])]);

    for (const webhook of result.rows) {
      const payload = {
        event,
        timestamp: new Date().toISOString(),
        data
      };

      const signature = crypto
        .createHmac('sha256', webhook.secret)
        .update(JSON.stringify(payload))
        .digest('hex');

      try {
        await fetch(webhook.url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-IPLoop-Signature': signature,
            'X-IPLoop-Event': event
          },
          body: JSON.stringify(payload),
          timeout: 5000
        });

        await query(
          'UPDATE webhooks SET last_triggered_at = NOW(), failure_count = 0 WHERE id = $1',
          [webhook.id]
        );
      } catch (error) {
        await query(
          'UPDATE webhooks SET failure_count = failure_count + 1 WHERE id = $1',
          [webhook.id]
        );
        logger.error('Webhook delivery failed', { webhookId: webhook.id, error: error.message });
      }
    }
  } catch (error) {
    logger.error('Error triggering webhooks', { userId, event, error: error.message });
  }
};

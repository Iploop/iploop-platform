const express = require('express');
const { query } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

// Get usage summary
router.get('/summary', authenticateToken, asyncHandler(async (req, res) => {
  const { days = 30 } = req.query;
  
  // Get user's plan info
  const planResult = await query(`
    SELECT up.gb_balance, up.gb_used, p.name as plan_name, p.monthly_gb
    FROM user_plans up
    JOIN plans p ON up.plan_id = p.id
    WHERE up.user_id = $1 AND up.status = 'active'
  `, [req.user.id]);

  const plan = planResult.rows[0] || {
    gb_balance: 0,
    gb_used: 0,
    plan_name: 'Free',
    monthly_gb: 0
  };

  // Get usage stats for the period
  const usageResult = await query(`
    SELECT 
      COUNT(*) as total_requests,
      SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) as successful_requests,
      SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as failed_requests,
      COALESCE(SUM(bytes_transferred), 0) as total_bytes,
      COALESCE(AVG(response_time_ms), 0) as avg_response_time
    FROM usage_logs
    WHERE user_id = $1 
    AND created_at >= NOW() - INTERVAL '${parseInt(days)} days'
  `, [req.user.id]);

  const usage = usageResult.rows[0];
  const totalBytes = parseInt(usage.total_bytes) || 0;
  const totalGB = totalBytes / (1024 * 1024 * 1024);

  res.json({
    plan: {
      name: plan.plan_name,
      monthlyGb: parseFloat(plan.monthly_gb) || 0,
      gbBalance: parseFloat(plan.gb_balance) || 0,
      gbUsed: parseFloat(plan.gb_used) || 0
    },
    period: {
      days: parseInt(days),
      startDate: new Date(Date.now() - days * 24 * 60 * 60 * 1000).toISOString(),
      endDate: new Date().toISOString()
    },
    stats: {
      totalRequests: parseInt(usage.total_requests) || 0,
      successfulRequests: parseInt(usage.successful_requests) || 0,
      failedRequests: parseInt(usage.failed_requests) || 0,
      successRate: usage.total_requests > 0 
        ? ((usage.successful_requests / usage.total_requests) * 100).toFixed(2) 
        : 0,
      totalBytesTransferred: totalBytes,
      totalGbTransferred: totalGB.toFixed(4),
      avgResponseTimeMs: Math.round(parseFloat(usage.avg_response_time) || 0)
    }
  });
}));

// Get daily usage breakdown
router.get('/daily', authenticateToken, asyncHandler(async (req, res) => {
  const { days = 30 } = req.query;

  const result = await query(`
    SELECT 
      DATE(created_at) as date,
      COUNT(*) as requests,
      SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) as successful,
      COALESCE(SUM(bytes_transferred), 0) as bytes,
      COALESCE(AVG(response_time_ms), 0) as avg_response_time
    FROM usage_logs
    WHERE user_id = $1 
    AND created_at >= NOW() - INTERVAL '${parseInt(days)} days'
    GROUP BY DATE(created_at)
    ORDER BY date DESC
  `, [req.user.id]);

  res.json({
    daily: result.rows.map(row => ({
      date: row.date,
      requests: parseInt(row.requests),
      successful: parseInt(row.successful),
      bytesTransferred: parseInt(row.bytes),
      mbTransferred: (parseInt(row.bytes) / (1024 * 1024)).toFixed(2),
      avgResponseTimeMs: Math.round(parseFloat(row.avg_response_time))
    }))
  });
}));

// Get usage by country
router.get('/by-country', authenticateToken, asyncHandler(async (req, res) => {
  const { days = 30 } = req.query;

  const result = await query(`
    SELECT 
      country_code,
      COUNT(*) as requests,
      COALESCE(SUM(bytes_transferred), 0) as bytes
    FROM usage_logs
    WHERE user_id = $1 
    AND created_at >= NOW() - INTERVAL '${parseInt(days)} days'
    GROUP BY country_code
    ORDER BY requests DESC
  `, [req.user.id]);

  res.json({
    byCountry: result.rows.map(row => ({
      country: row.country_code || 'Unknown',
      requests: parseInt(row.requests),
      bytesTransferred: parseInt(row.bytes),
      mbTransferred: (parseInt(row.bytes) / (1024 * 1024)).toFixed(2)
    }))
  });
}));

// Get usage by API key
router.get('/by-key', authenticateToken, asyncHandler(async (req, res) => {
  const { days = 30 } = req.query;

  const result = await query(`
    SELECT 
      ul.api_key_id,
      ak.name as key_name,
      COUNT(*) as requests,
      COALESCE(SUM(ul.bytes_transferred), 0) as bytes,
      COALESCE(AVG(ul.response_time_ms), 0) as avg_response_time
    FROM usage_logs ul
    LEFT JOIN api_keys ak ON ul.api_key_id = ak.id
    WHERE ul.user_id = $1 
    AND ul.created_at >= NOW() - INTERVAL '${parseInt(days)} days'
    GROUP BY ul.api_key_id, ak.name
    ORDER BY requests DESC
  `, [req.user.id]);

  res.json({
    byKey: result.rows.map(row => ({
      keyId: row.api_key_id,
      keyName: row.key_name || 'Unknown',
      requests: parseInt(row.requests),
      bytesTransferred: parseInt(row.bytes),
      mbTransferred: (parseInt(row.bytes) / (1024 * 1024)).toFixed(2),
      avgResponseTimeMs: Math.round(parseFloat(row.avg_response_time))
    }))
  });
}));

// Get recent requests (for debugging)
router.get('/recent', authenticateToken, asyncHandler(async (req, res) => {
  const { limit = 50 } = req.query;

  const result = await query(`
    SELECT 
      id, 
      target_url, 
      country_code, 
      status_code, 
      bytes_transferred, 
      response_time_ms, 
      created_at
    FROM usage_logs
    WHERE user_id = $1
    ORDER BY created_at DESC
    LIMIT $2
  `, [req.user.id, Math.min(parseInt(limit), 100)]);

  res.json({
    requests: result.rows.map(row => ({
      id: row.id,
      targetUrl: row.target_url,
      country: row.country_code,
      statusCode: row.status_code,
      bytesTransferred: row.bytes_transferred,
      responseTimeMs: row.response_time_ms,
      timestamp: row.created_at
    }))
  });
}));

module.exports = router;

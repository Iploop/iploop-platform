const express = require('express');
const { query } = require('../config/database');
const { APIError, asyncHandler } = require('../middleware/errorHandler');
const { authenticateToken } = require('../middleware/auth');
const logger = require('../utils/logger');

const router = express.Router();

const NODE_REGISTRATION_URL = process.env.NODE_REGISTRATION_URL || 'http://localhost:8001';

// Get available nodes (for proxy routing)
router.get('/', authenticateToken, asyncHandler(async (req, res) => {
  const { country, city, status = 'available' } = req.query;

  try {
    let url = `${NODE_REGISTRATION_URL}/nodes?status=${status}`;
    if (country) url += `&country=${country}`;
    if (city) url += `&city=${city}`;

    const response = await fetch(url);
    if (!response.ok) {
      throw new Error('Failed to fetch nodes');
    }

    const data = await response.json();

    res.json({
      nodes: data.nodes.map(node => ({
        id: node.id,
        country: node.country,
        countryName: node.country_name,
        city: node.city,
        region: node.region,
        isp: node.isp,
        connectionType: node.connection_type,
        deviceType: node.device_type,
        qualityScore: node.quality_score,
        status: node.status
      })),
      total: data.count
    });
  } catch (error) {
    logger.error('Error fetching nodes:', error);
    throw new APIError('Failed to fetch available nodes', 502);
  }
}));

// Get node statistics (summary)
router.get('/stats', authenticateToken, asyncHandler(async (req, res) => {
  try {
    const response = await fetch(`${NODE_REGISTRATION_URL}/stats`);
    if (!response.ok) {
      throw new Error('Failed to fetch stats');
    }

    const stats = await response.json();

    res.json({
      totalNodes: stats.total_nodes || 0,
      activeNodes: stats.active_nodes || 0,
      countries: stats.country_breakdown || {},
      deviceTypes: stats.device_types || {},
      connectionTypes: stats.connection_types || {},
      averageQuality: stats.average_quality || 0
    });
  } catch (error) {
    logger.error('Error fetching node stats:', error);
    throw new APIError('Failed to fetch node statistics', 502);
  }
}));

// Get available countries
router.get('/countries', authenticateToken, asyncHandler(async (req, res) => {
  try {
    const response = await fetch(`${NODE_REGISTRATION_URL}/stats`);
    if (!response.ok) {
      throw new Error('Failed to fetch stats');
    }

    const stats = await response.json();
    const countries = stats.country_breakdown || {};

    const countryList = Object.entries(countries).map(([code, count]) => ({
      code,
      name: getCountryName(code),
      nodeCount: count
    })).sort((a, b) => b.nodeCount - a.nodeCount);

    res.json({
      countries: countryList,
      total: countryList.length
    });
  } catch (error) {
    logger.error('Error fetching countries:', error);
    throw new APIError('Failed to fetch available countries', 502);
  }
}));

// Helper function for country names
function getCountryName(code) {
  const names = {
    IL: 'Israel',
    US: 'United States',
    UK: 'United Kingdom',
    GB: 'United Kingdom',
    DE: 'Germany',
    FR: 'France',
    CA: 'Canada',
    AU: 'Australia',
    JP: 'Japan',
    KR: 'South Korea',
    BR: 'Brazil',
    IN: 'India',
    IT: 'Italy',
    ES: 'Spain',
    NL: 'Netherlands',
    PL: 'Poland',
    RU: 'Russia',
    CN: 'China',
    MX: 'Mexico',
    AR: 'Argentina'
  };
  return names[code] || code;
}

module.exports = router;

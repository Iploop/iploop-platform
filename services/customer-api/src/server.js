const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');
const rateLimit = require('express-rate-limit');
require('dotenv').config();

const logger = require('./utils/logger');
const { connectDatabase, connectRedis } = require('./config/database');
const authRoutes = require('./routes/auth');
const proxyRoutes = require('./routes/proxy');
const usageRoutes = require('./routes/usage');
const nodesRoutes = require('./routes/nodes');
const adminRoutes = require('./routes/admin');
const webhooksRoutes = require('./routes/webhooks');
// TODO: Implement these routes
// const billingRoutes = require('./routes/billing');
const { errorHandler } = require('./middleware/errorHandler');

const app = express();
const PORT = process.env.PORT || 8002;

// Security middleware
app.use(helmet());
app.use(cors({
  origin: process.env.ALLOWED_ORIGINS?.split(',') || ['http://localhost:3000'],
  credentials: true
}));

// Rate limiting
const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100, // limit each IP to 100 requests per windowMs
  message: 'Too many requests from this IP',
  standardHeaders: true,
  legacyHeaders: false,
});
app.use(limiter);

// Logging middleware
app.use(morgan('combined', { stream: { write: message => logger.info(message.trim()) } }));

// Body parsing middleware
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// Initialize database connections
async function initializeConnections() {
  try {
    await connectDatabase();
    await connectRedis();
    logger.info('Database connections established');
  } catch (error) {
    logger.error('Failed to establish database connections:', error);
    process.exit(1);
  }
}

// Health check endpoint
app.get('/health', async (req, res) => {
  try {
    const { db, redis } = require('./config/database');
    
    // Check database
    await db.query('SELECT 1');
    
    // Check Redis
    await redis.ping();
    
    res.json({
      status: 'healthy',
      timestamp: new Date().toISOString(),
      service: 'customer-api',
      version: '1.0.0'
    });
  } catch (error) {
    logger.error('Health check failed:', error);
    res.status(503).json({
      status: 'unhealthy',
      error: error.message,
      timestamp: new Date().toISOString()
    });
  }
});

// API routes
app.use('/api/v1/auth', authRoutes);
app.use('/api/v1/proxy', proxyRoutes);
app.use('/api/v1/usage', usageRoutes);
app.use('/api/v1/nodes', nodesRoutes);
app.use('/api/v1/admin', adminRoutes);
app.use('/api/v1/webhooks', webhooksRoutes);
// TODO: Implement these routes
// app.use('/api/v1/billing', billingRoutes);

// Root endpoint
app.get('/', (req, res) => {
  res.json({
    service: 'IPLoop Customer API',
    version: '1.0.0',
    timestamp: new Date().toISOString(),
    endpoints: {
      auth: '/api/v1/auth',
      proxy: '/api/v1/proxy',
      usage: '/api/v1/usage',
      billing: '/api/v1/billing',
      network: '/api/v1/network'
    }
  });
});

// Error handling middleware
app.use(errorHandler);

// 404 handler
app.use('*', (req, res) => {
  res.status(404).json({
    error: 'Endpoint not found',
    path: req.originalUrl,
    method: req.method
  });
});

// Server variable for graceful shutdown
let server;

// Graceful shutdown
process.on('SIGTERM', () => {
  logger.info('SIGTERM received, shutting down gracefully');
  if (server) {
    server.close(() => {
      logger.info('Process terminated');
      process.exit(0);
    });
  } else {
    process.exit(0);
  }
});

process.on('SIGINT', () => {
  logger.info('SIGINT received, shutting down gracefully');
  if (server) {
    server.close(() => {
      logger.info('Process terminated');
      process.exit(0);
    });
  } else {
    process.exit(0);
  }
});

// Start server
async function startServer() {
  try {
    await initializeConnections();
    
    server = app.listen(PORT, '0.0.0.0', () => {
      logger.info(`Customer API server running on port ${PORT}`);
      logger.info(`Environment: ${process.env.NODE_ENV || 'development'}`);
    });
    
  } catch (error) {
    logger.error('Failed to start server:', error);
    process.exit(1);
  }
}

if (require.main === module) {
  startServer();
}

module.exports = app;
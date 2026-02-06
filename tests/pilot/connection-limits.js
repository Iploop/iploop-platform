#!/usr/bin/env node
/**
 * Connection Limits & Pool Exhaustion Test
 * 
 * Tests:
 * - Maximum concurrent connections
 * - Connection pool behavior
 * - What happens at saturation
 * - Connection reuse (keep-alive)
 */

const http = require('http');
const net = require('net');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

const metrics = {
  attempted: 0,
  connected: 0,
  rejected: 0,
  timeout: 0,
  errors: {},
  maxConcurrent: 0,
  currentConcurrent: 0
};

// Raw TCP connection test
function testRawConnection() {
  return new Promise((resolve) => {
    metrics.attempted++;
    metrics.currentConcurrent++;
    metrics.maxConcurrent = Math.max(metrics.maxConcurrent, metrics.currentConcurrent);
    
    const socket = new net.Socket();
    socket.setTimeout(10000);
    
    socket.connect(CONFIG.proxyPort, CONFIG.proxyHost, () => {
      metrics.connected++;
      socket.end();
      metrics.currentConcurrent--;
      resolve({ success: true });
    });
    
    socket.on('error', (err) => {
      metrics.errors[err.code] = (metrics.errors[err.code] || 0) + 1;
      metrics.currentConcurrent--;
      resolve({ success: false, error: err.code });
    });
    
    socket.on('timeout', () => {
      metrics.timeout++;
      socket.destroy();
      metrics.currentConcurrent--;
      resolve({ success: false, error: 'TIMEOUT' });
    });
  });
}

// HTTP request that holds connection
function holdConnection(durationMs) {
  return new Promise((resolve) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    metrics.attempted++;
    metrics.currentConcurrent++;
    metrics.maxConcurrent = Math.max(metrics.maxConcurrent, metrics.currentConcurrent);
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: `https://httpbin.org/delay/${Math.ceil(durationMs/1000)}`,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        'Connection': 'keep-alive'
      },
      timeout: durationMs + 30000
    };
    
    const req = http.request(options, (res) => {
      res.on('data', () => {});
      res.on('end', () => {
        metrics.connected++;
        metrics.currentConcurrent--;
        resolve({ success: true });
      });
    });
    
    req.on('error', (err) => {
      metrics.errors[err.code] = (metrics.errors[err.code] || 0) + 1;
      metrics.currentConcurrent--;
      resolve({ success: false, error: err.code });
    });
    
    req.on('timeout', () => {
      metrics.timeout++;
      req.destroy();
      metrics.currentConcurrent--;
      resolve({ success: false, error: 'TIMEOUT' });
    });
    
    req.end();
  });
}

async function testConnectionLimits() {
  console.log('🔌 Connection Limits Test');
  console.log('='.repeat(60));
  
  // Test 1: Rapid connection burst
  console.log('\n--- Test 1: Rapid Connection Burst (100 simultaneous) ---');
  metrics.attempted = 0;
  metrics.connected = 0;
  metrics.maxConcurrent = 0;
  
  const burstPromises = [];
  for (let i = 0; i < 100; i++) {
    burstPromises.push(testRawConnection());
  }
  await Promise.all(burstPromises);
  
  console.log(`Attempted: ${metrics.attempted}`);
  console.log(`Connected: ${metrics.connected}`);
  console.log(`Max concurrent: ${metrics.maxConcurrent}`);
  console.log(`Errors: ${JSON.stringify(metrics.errors)}`);
  
  // Test 2: Connection holding
  console.log('\n--- Test 2: Connection Holding (20 x 5s each) ---');
  metrics.attempted = 0;
  metrics.connected = 0;
  metrics.maxConcurrent = 0;
  metrics.currentConcurrent = 0;
  metrics.errors = {};
  
  const holdPromises = [];
  for (let i = 0; i < 20; i++) {
    holdPromises.push(holdConnection(5000));
    await new Promise(r => setTimeout(r, 100)); // Stagger
  }
  
  // Monitor progress
  const monitor = setInterval(() => {
    console.log(`  Active: ${metrics.currentConcurrent} | Completed: ${metrics.connected}`);
  }, 2000);
  
  await Promise.all(holdPromises);
  clearInterval(monitor);
  
  console.log(`\nCompleted: ${metrics.connected}/20`);
  console.log(`Max concurrent held: ${metrics.maxConcurrent}`);
  
  // Test 3: Escalating load
  console.log('\n--- Test 3: Escalating Load (find breaking point) ---');
  
  for (const count of [50, 100, 200, 500]) {
    metrics.attempted = 0;
    metrics.connected = 0;
    metrics.errors = {};
    
    process.stdout.write(`Testing ${count} connections... `);
    
    const promises = [];
    for (let i = 0; i < count; i++) {
      promises.push(testRawConnection());
    }
    await Promise.all(promises);
    
    const successRate = (metrics.connected / count * 100).toFixed(1);
    console.log(`${successRate}% success`);
    
    if (metrics.connected < count * 0.9) {
      console.log(`  ⚠️  Breaking point around ${count} connections`);
      console.log(`  Errors: ${JSON.stringify(metrics.errors)}`);
      break;
    }
    
    await new Promise(r => setTimeout(r, 1000)); // Cool down
  }
  
  console.log('\n' + '='.repeat(60));
  console.log('Connection limits test complete');
}

testConnectionLimits().catch(console.error);

#!/usr/bin/env node
/**
 * IPLoop Sticky Session Stress Test
 * 
 * Tests session stickiness under heavy load:
 * - Many concurrent sticky sessions
 * - Session persistence over time
 * - Session behavior when peers disconnect
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key',
  
  // Test params
  concurrentSessions: parseInt(process.env.SESSIONS || '50'),
  requestsPerSession: parseInt(process.env.REQUESTS || '10'),
  sessionLifetime: parseInt(process.env.LIFETIME || '60'), // seconds
  delayBetweenRequests: 1000 // ms
};

const metrics = {
  sessionsStarted: 0,
  sessionsCompleted: 0,
  sessionsFailed: 0,
  totalRequests: 0,
  stickySuccess: 0,    // Same IP throughout session
  stickyFailure: 0,    // IP changed mid-session
  requestErrors: 0,
  ipChanges: []        // Track when IPs changed
};

function makeRequest(sessionId) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}-session-${sessionId}-sesstype-sticky-lifetime-${CONFIG.sessionLifetime}`;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/ip',
      headers: {
        'Host': 'httpbin.org',
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 30000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const json = JSON.parse(data);
          resolve({ ip: json.origin, status: res.statusCode });
        } catch (e) {
          reject(new Error('Parse error'));
        }
      });
    });
    
    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    req.end();
  });
}

async function runSession(sessionNum) {
  const sessionId = `stress_${Date.now()}_${sessionNum}`;
  const ips = [];
  let errors = 0;
  
  metrics.sessionsStarted++;
  
  for (let i = 0; i < CONFIG.requestsPerSession; i++) {
    try {
      const result = await makeRequest(sessionId);
      metrics.totalRequests++;
      ips.push(result.ip);
      
      // Check for IP change
      if (ips.length > 1 && ips[ips.length - 1] !== ips[ips.length - 2]) {
        metrics.ipChanges.push({
          session: sessionNum,
          request: i,
          from: ips[ips.length - 2],
          to: ips[ips.length - 1]
        });
      }
    } catch (err) {
      errors++;
      metrics.requestErrors++;
    }
    
    // Delay between requests
    await new Promise(r => setTimeout(r, CONFIG.delayBetweenRequests));
  }
  
  // Evaluate session
  const uniqueIps = [...new Set(ips)];
  if (uniqueIps.length === 1 && ips.length === CONFIG.requestsPerSession) {
    metrics.stickySuccess++;
    metrics.sessionsCompleted++;
  } else if (uniqueIps.length > 1) {
    metrics.stickyFailure++;
    metrics.sessionsFailed++;
  } else if (errors > 0) {
    metrics.sessionsFailed++;
  } else {
    metrics.sessionsCompleted++;
  }
  
  return { sessionNum, ips, uniqueIps: uniqueIps.length, errors };
}

async function runStickySressTest() {
  console.log('🔗 IPLoop Sticky Session Stress Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Concurrent sessions: ${CONFIG.concurrentSessions}`);
  console.log(`Requests per session: ${CONFIG.requestsPerSession}`);
  console.log(`Session lifetime: ${CONFIG.sessionLifetime}s`);
  console.log('='.repeat(60));
  console.log('');
  
  const startTime = Date.now();
  
  // Progress reporter
  const progressInterval = setInterval(() => {
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    console.log(`[${elapsed}s] Sessions: ${metrics.sessionsCompleted}/${CONFIG.concurrentSessions} | Requests: ${metrics.totalRequests} | Sticky OK: ${metrics.stickySuccess} | Failed: ${metrics.stickyFailure}`);
  }, 5000);
  
  // Run sessions concurrently
  const sessionPromises = [];
  for (let i = 0; i < CONFIG.concurrentSessions; i++) {
    // Stagger session starts
    await new Promise(r => setTimeout(r, 50));
    sessionPromises.push(runSession(i));
  }
  
  const results = await Promise.all(sessionPromises);
  clearInterval(progressInterval);
  
  const elapsed = (Date.now() - startTime) / 1000;
  
  // Report
  console.log('\n' + '='.repeat(60));
  console.log('📊 STICKY SESSION STRESS RESULTS');
  console.log('='.repeat(60));
  console.log(`Duration: ${elapsed.toFixed(1)}s`);
  console.log(`Total sessions: ${CONFIG.concurrentSessions}`);
  console.log(`Total requests: ${metrics.totalRequests}`);
  console.log('');
  console.log('Session Results:');
  console.log(`  ✅ Sticky maintained: ${metrics.stickySuccess} (${(metrics.stickySuccess/CONFIG.concurrentSessions*100).toFixed(1)}%)`);
  console.log(`  ❌ Sticky broken: ${metrics.stickyFailure}`);
  console.log(`  ⚠️  Request errors: ${metrics.requestErrors}`);
  
  if (metrics.ipChanges.length > 0) {
    console.log('\n🔄 IP Changes Detected:');
    metrics.ipChanges.slice(0, 10).forEach(change => {
      console.log(`  Session ${change.session}, request ${change.request}: ${change.from} → ${change.to}`);
    });
    if (metrics.ipChanges.length > 10) {
      console.log(`  ... and ${metrics.ipChanges.length - 10} more`);
    }
  }
  
  // Verdict
  console.log('\n' + '='.repeat(60));
  if (metrics.stickySuccess === CONFIG.concurrentSessions) {
    console.log('✅ PASS: All sessions maintained sticky IPs!');
  } else if (metrics.stickySuccess / CONFIG.concurrentSessions >= 0.95) {
    console.log('⚠️  WARN: 95%+ sessions OK, some stickiness issues');
  } else {
    console.log('❌ FAIL: Significant session stickiness problems');
  }
}

runStickySressTest().catch(console.error);

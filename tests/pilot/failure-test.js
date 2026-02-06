#!/usr/bin/env node
/**
 * IPLoop Failure & Recovery Test
 * 
 * Tests error handling and recovery:
 * - Target timeout handling
 * - Invalid target handling
 * - Auth failures
 * - Graceful degradation
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeRequest(url, auth, timeout = 30000) {
  return new Promise((resolve) => {
    const startTime = Date.now();
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({
          success: true,
          statusCode: res.statusCode,
          elapsed: Date.now() - startTime,
          data: data.substring(0, 200)
        });
      });
    });
    
    req.on('error', (err) => {
      resolve({
        success: false,
        error: err.message,
        code: err.code,
        elapsed: Date.now() - startTime
      });
    });
    
    req.on('timeout', () => {
      req.destroy();
      resolve({
        success: false,
        error: 'timeout',
        code: 'ETIMEDOUT',
        elapsed: Date.now() - startTime
      });
    });
    
    req.end();
  });
}

const TESTS = [
  {
    name: 'Valid request (baseline)',
    url: 'https://httpbin.org/ip',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { statusCode: 200 }
  },
  {
    name: 'Target timeout (10s delay, 5s timeout)',
    url: 'https://httpbin.org/delay/10',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    timeout: 5000,
    expect: { error: true }
  },
  {
    name: 'Target 404',
    url: 'https://httpbin.org/status/404',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { statusCode: 404 }
  },
  {
    name: 'Target 500',
    url: 'https://httpbin.org/status/500',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { statusCode: 500 }
  },
  {
    name: 'Invalid auth (wrong password)',
    url: 'https://httpbin.org/ip',
    auth: () => `${CONFIG.proxyUser}:wrong_password`,
    expect: { statusCode: 407 } // Or 401/403 depending on implementation
  },
  {
    name: 'Invalid auth (empty)',
    url: 'https://httpbin.org/ip',
    auth: () => '',
    expect: { error: true }
  },
  {
    name: 'Non-existent domain',
    url: 'http://this-domain-does-not-exist-12345.com/',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { error: true }
  },
  {
    name: 'Connection refused port',
    url: 'http://httpbin.org:12345/',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    timeout: 10000,
    expect: { error: true }
  },
  {
    name: 'Large redirect chain',
    url: 'https://httpbin.org/redirect/5',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { statusCode: 200 }
  },
  {
    name: 'Recovery after failure',
    url: 'https://httpbin.org/ip',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: { statusCode: 200 }
  }
];

async function runFailureTests() {
  console.log('🔥 IPLoop Failure & Recovery Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  console.log('');
  
  const results = [];
  
  for (const test of TESTS) {
    process.stdout.write(`${test.name.padEnd(40)}... `);
    
    const result = await makeRequest(
      test.url, 
      test.auth(), 
      test.timeout || 30000
    );
    
    let passed = false;
    let detail = '';
    
    if (test.expect.error) {
      // Expected an error
      passed = !result.success || result.statusCode >= 400;
      detail = result.error || `HTTP ${result.statusCode}`;
    } else if (test.expect.statusCode) {
      // Expected specific status code
      passed = result.statusCode === test.expect.statusCode;
      detail = result.statusCode ? `HTTP ${result.statusCode}` : result.error;
    }
    
    results.push({ ...test, result, passed });
    
    if (passed) {
      console.log(`✅ ${detail} (${result.elapsed}ms)`);
    } else {
      console.log(`❌ Got: ${detail}, Expected: ${JSON.stringify(test.expect)}`);
    }
  }
  
  // Summary
  console.log('\n' + '='.repeat(60));
  console.log('FAILURE HANDLING SUMMARY');
  console.log('='.repeat(60));
  
  const passed = results.filter(r => r.passed).length;
  console.log(`Tests passed: ${passed}/${TESTS.length}`);
  
  if (passed === TESTS.length) {
    console.log('✅ All failure scenarios handled correctly!');
  } else {
    console.log('\nFailed tests:');
    results.filter(r => !r.passed).forEach(r => {
      console.log(`  ❌ ${r.name}`);
    });
  }
  
  // Check recovery
  const lastTest = results[results.length - 1];
  if (lastTest.name.includes('Recovery') && lastTest.passed) {
    console.log('\n✅ Proxy recovered after failures');
  } else if (lastTest.name.includes('Recovery')) {
    console.log('\n⚠️  Proxy may not have recovered properly after failures');
  }
}

runFailureTests().catch(console.error);

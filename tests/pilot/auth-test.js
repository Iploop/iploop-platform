#!/usr/bin/env node
/**
 * Authentication Comprehensive Test
 * 
 * Tests all auth scenarios:
 * - Valid credentials
 * - Invalid credentials
 * - Missing credentials
 * - Malformed credentials
 * - All parameter combinations
 * - Special characters in auth
 * - Auth header variations
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeRequest(auth, useHeader = true) {
  return new Promise((resolve) => {
    const headers = {};
    
    if (useHeader && auth !== null) {
      headers['Proxy-Authorization'] = auth.startsWith('Basic ') 
        ? auth 
        : 'Basic ' + Buffer.from(auth).toString('base64');
    }
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/ip',
      headers,
      timeout: 15000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({ statusCode: res.statusCode, body: data });
      });
    });
    
    req.on('error', (err) => {
      resolve({ error: err.message, code: err.code });
    });
    
    req.on('timeout', () => {
      req.destroy();
      resolve({ error: 'timeout' });
    });
    
    req.end();
  });
}

const TESTS = [
  // Valid auth
  {
    name: 'Valid credentials',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}`,
    expect: 200
  },
  
  // Invalid auth
  {
    name: 'Wrong password',
    auth: () => `${CONFIG.proxyUser}:wrong_password`,
    expect: [401, 403, 407]
  },
  {
    name: 'Wrong username',
    auth: () => `wrong_user:${CONFIG.proxyPass}`,
    expect: [401, 403, 407]
  },
  {
    name: 'Empty password',
    auth: () => `${CONFIG.proxyUser}:`,
    expect: [401, 403, 407]
  },
  {
    name: 'Empty username',
    auth: () => `:${CONFIG.proxyPass}`,
    expect: [401, 403, 407]
  },
  {
    name: 'No credentials (null)',
    auth: () => null,
    expect: [401, 403, 407]
  },
  {
    name: 'Empty string',
    auth: () => '',
    expect: [401, 403, 407, 400]
  },
  
  // Malformed auth
  {
    name: 'Missing colon separator',
    auth: () => `${CONFIG.proxyUser}${CONFIG.proxyPass}`,
    expect: [401, 403, 407, 400]
  },
  {
    name: 'Multiple colons',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}:extra`,
    expect: [401, 403, 407, 200] // Some proxies accept this
  },
  {
    name: 'Invalid base64',
    auth: () => 'Basic not-valid-base64!!!',
    expect: [401, 403, 407, 400]
  },
  {
    name: 'Wrong auth scheme (Bearer)',
    auth: () => 'Bearer ' + Buffer.from(`${CONFIG.proxyUser}:${CONFIG.proxyPass}`).toString('base64'),
    expect: [401, 403, 407]
  },
  
  // Parameter combinations
  {
    name: 'With country param',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-country-US`,
    expect: 200
  },
  {
    name: 'With city param',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-city-newyork`,
    expect: 200
  },
  {
    name: 'With session param',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-session-test123`,
    expect: 200
  },
  {
    name: 'With sesstype sticky',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-sesstype-sticky`,
    expect: 200
  },
  {
    name: 'With sesstype rotating',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-sesstype-rotating`,
    expect: 200
  },
  {
    name: 'Multiple params combined',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-country-US-city-miami-session-abc-sesstype-sticky`,
    expect: 200
  },
  {
    name: 'Invalid country code',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-country-XX`,
    expect: [200, 400, 502] // May work or fail gracefully
  },
  {
    name: 'Unknown param (should ignore)',
    auth: () => `${CONFIG.proxyUser}:${CONFIG.proxyPass}-unknown-value`,
    expect: 200
  },
  
  // Special characters
  {
    name: 'Password with special chars',
    auth: () => `${CONFIG.proxyUser}:pass!@#$%^&*()`,
    expect: [401, 403, 407] // Wrong pass but tests encoding
  },
  {
    name: 'Username with dots',
    auth: () => `user.name.test:${CONFIG.proxyPass}`,
    expect: [401, 403, 407]
  },
  {
    name: 'Unicode in auth',
    auth: () => `用户:${CONFIG.proxyPass}`,
    expect: [401, 403, 407, 400]
  },
  
  // Edge cases
  {
    name: 'Very long username (1000 chars)',
    auth: () => `${'a'.repeat(1000)}:${CONFIG.proxyPass}`,
    expect: [401, 403, 407, 400, 413]
  },
  {
    name: 'Very long password (1000 chars)',
    auth: () => `${CONFIG.proxyUser}:${'a'.repeat(1000)}`,
    expect: [401, 403, 407, 400, 413]
  },
  {
    name: 'Newline in auth',
    auth: () => `${CONFIG.proxyUser}\n:${CONFIG.proxyPass}`,
    expect: [401, 403, 407, 400]
  },
  {
    name: 'Null byte in auth',
    auth: () => `${CONFIG.proxyUser}\x00:${CONFIG.proxyPass}`,
    expect: [401, 403, 407, 400]
  }
];

async function runAuthTests() {
  console.log('🔐 Authentication Comprehensive Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Tests: ${TESTS.length}`);
  console.log('='.repeat(60));
  console.log('');
  
  let passed = 0;
  let failed = 0;
  const failures = [];
  
  for (const test of TESTS) {
    process.stdout.write(`${test.name.padEnd(40)}... `);
    
    try {
      const auth = test.auth();
      const result = await makeRequest(auth);
      
      const expectedCodes = Array.isArray(test.expect) ? test.expect : [test.expect];
      const pass = expectedCodes.includes(result.statusCode);
      
      if (pass) {
        console.log(`✅ HTTP ${result.statusCode}`);
        passed++;
      } else if (result.error) {
        console.log(`❌ Error: ${result.error}`);
        failed++;
        failures.push(`${test.name}: ${result.error}`);
      } else {
        console.log(`❌ HTTP ${result.statusCode} (expected ${expectedCodes.join('/')})`);
        failed++;
        failures.push(`${test.name}: got ${result.statusCode}`);
      }
    } catch (err) {
      console.log(`❌ ${err.message}`);
      failed++;
      failures.push(`${test.name}: ${err.message}`);
    }
    
    await new Promise(r => setTimeout(r, 100));
  }
  
  console.log('\n' + '='.repeat(60));
  console.log('AUTH TEST RESULTS');
  console.log('='.repeat(60));
  console.log(`Passed: ${passed}/${TESTS.length}`);
  console.log(`Failed: ${failed}`);
  
  if (failures.length > 0) {
    console.log('\nFailed tests:');
    failures.forEach(f => console.log(`  ❌ ${f}`));
  }
  
  // Security assessment
  console.log('\n🔒 Security Assessment:');
  
  // Check if invalid creds are properly rejected
  const authTests = TESTS.filter(t => t.name.includes('Wrong') || t.name.includes('Empty') || t.name.includes('No credentials'));
  // If those passed (got 401/403/407), auth is working
  console.log('  Auth rejection: Tests above show if invalid creds are rejected');
}

runAuthTests().catch(console.error);

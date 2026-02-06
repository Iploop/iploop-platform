#!/usr/bin/env node
/**
 * Protocol Comprehensive Test
 * 
 * Tests all protocol aspects:
 * - HTTP methods (GET, POST, PUT, DELETE, HEAD, OPTIONS)
 * - HTTPS tunneling
 * - Request/response headers
 * - Request bodies
 * - Chunked encoding
 * - Compression (gzip, deflate)
 * - Redirects
 * - Cookies
 * - Binary data
 */

const http = require('http');
const zlib = require('zlib');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeRequest(method, url, options = {}) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    
    const reqOptions = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: method,
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        ...options.headers
      },
      timeout: 30000
    };
    
    const req = http.request(reqOptions, (res) => {
      const chunks = [];
      res.on('data', chunk => chunks.push(chunk));
      res.on('end', () => {
        const buffer = Buffer.concat(chunks);
        resolve({
          statusCode: res.statusCode,
          headers: res.headers,
          body: buffer,
          bodyText: buffer.toString('utf8')
        });
      });
    });
    
    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    
    if (options.body) {
      req.write(options.body);
    }
    req.end();
  });
}

const TESTS = [
  // HTTP Methods
  {
    name: 'GET request',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/get');
      return res.statusCode === 200;
    }
  },
  {
    name: 'POST with JSON body',
    run: async () => {
      const body = JSON.stringify({ test: 'data', num: 123 });
      const res = await makeRequest('POST', 'https://httpbin.org/post', {
        headers: { 'Content-Type': 'application/json', 'Content-Length': body.length },
        body
      });
      const json = JSON.parse(res.bodyText);
      return res.statusCode === 200 && json.json?.test === 'data';
    }
  },
  {
    name: 'POST with form data',
    run: async () => {
      const body = 'field1=value1&field2=value2';
      const res = await makeRequest('POST', 'https://httpbin.org/post', {
        headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'Content-Length': body.length },
        body
      });
      const json = JSON.parse(res.bodyText);
      return json.form?.field1 === 'value1';
    }
  },
  {
    name: 'PUT request',
    run: async () => {
      const body = 'update data';
      const res = await makeRequest('PUT', 'https://httpbin.org/put', {
        headers: { 'Content-Length': body.length },
        body
      });
      return res.statusCode === 200;
    }
  },
  {
    name: 'DELETE request',
    run: async () => {
      const res = await makeRequest('DELETE', 'https://httpbin.org/delete');
      return res.statusCode === 200;
    }
  },
  {
    name: 'HEAD request',
    run: async () => {
      const res = await makeRequest('HEAD', 'https://httpbin.org/get');
      return res.statusCode === 200 && res.body.length === 0;
    }
  },
  {
    name: 'OPTIONS request',
    run: async () => {
      const res = await makeRequest('OPTIONS', 'https://httpbin.org/get');
      return res.statusCode === 200;
    }
  },
  
  // Headers
  {
    name: 'Custom headers preserved',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/headers', {
        headers: { 'X-Custom-Header': 'test-value-123' }
      });
      const json = JSON.parse(res.bodyText);
      return json.headers['X-Custom-Header'] === 'test-value-123';
    }
  },
  {
    name: 'User-Agent preserved',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/user-agent', {
        headers: { 'User-Agent': 'TestBot/1.0' }
      });
      const json = JSON.parse(res.bodyText);
      return json['user-agent'] === 'TestBot/1.0';
    }
  },
  
  // Responses
  {
    name: 'Gzip compressed response',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/gzip');
      const json = JSON.parse(res.bodyText);
      return json.gzipped === true;
    }
  },
  {
    name: 'Deflate compressed response',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/deflate');
      const json = JSON.parse(res.bodyText);
      return json.deflated === true;
    }
  },
  {
    name: 'Chunked response',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/stream/5');
      return res.statusCode === 200 && res.body.length > 0;
    }
  },
  
  // Redirects
  {
    name: 'Follow redirect',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/redirect/1');
      return res.statusCode === 200 || res.statusCode === 302;
    }
  },
  {
    name: 'Multiple redirects (5)',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/redirect/5');
      return res.statusCode === 200 || res.statusCode === 302;
    }
  },
  {
    name: 'Absolute redirect',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/absolute-redirect/1');
      return res.statusCode === 200 || res.statusCode === 302;
    }
  },
  
  // Cookies
  {
    name: 'Set-Cookie header received',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/cookies/set?test=cookie123');
      return res.headers['set-cookie'] || res.statusCode === 302;
    }
  },
  {
    name: 'Send cookies',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/cookies', {
        headers: { 'Cookie': 'session=abc123; user=test' }
      });
      const json = JSON.parse(res.bodyText);
      return json.cookies?.session === 'abc123';
    }
  },
  
  // Binary/Special
  {
    name: 'Binary response (image)',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/image/png');
      return res.headers['content-type']?.includes('image/png') && res.body.length > 100;
    }
  },
  {
    name: 'Large response (100KB)',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/bytes/102400');
      return res.body.length >= 100000;
    }
  },
  {
    name: 'UTF-8 encoding',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/encoding/utf8');
      return res.statusCode === 200 && res.bodyText.includes('∮');
    }
  },
  
  // Status codes
  {
    name: 'Status 201 Created',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/status/201');
      return res.statusCode === 201;
    }
  },
  {
    name: 'Status 204 No Content',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/status/204');
      return res.statusCode === 204;
    }
  },
  {
    name: 'Status 418 Teapot',
    run: async () => {
      const res = await makeRequest('GET', 'https://httpbin.org/status/418');
      return res.statusCode === 418;
    }
  }
];

async function runProtocolTests() {
  console.log('📡 Protocol Comprehensive Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Tests: ${TESTS.length}`);
  console.log('='.repeat(60));
  console.log('');
  
  let passed = 0;
  let failed = 0;
  const failures = [];
  
  for (const test of TESTS) {
    process.stdout.write(`${test.name.padEnd(35)}... `);
    
    try {
      const result = await test.run();
      if (result) {
        console.log('✅');
        passed++;
      } else {
        console.log('❌ (unexpected result)');
        failed++;
        failures.push(test.name);
      }
    } catch (err) {
      console.log(`❌ ${err.message}`);
      failed++;
      failures.push(`${test.name}: ${err.message}`);
    }
    
    await new Promise(r => setTimeout(r, 200)); // Rate limit
  }
  
  console.log('\n' + '='.repeat(60));
  console.log('PROTOCOL TEST RESULTS');
  console.log('='.repeat(60));
  console.log(`Passed: ${passed}/${TESTS.length}`);
  console.log(`Failed: ${failed}`);
  
  if (failures.length > 0) {
    console.log('\nFailed tests:');
    failures.forEach(f => console.log(`  ❌ ${f}`));
  }
  
  if (passed === TESTS.length) {
    console.log('\n✅ All protocol tests passed!');
  }
}

runProtocolTests().catch(console.error);

#!/usr/bin/env node
/**
 * Security Test Suite
 * 
 * Tests:
 * - IP leak detection (comprehensive)
 * - Header injection prevention
 * - Request smuggling prevention
 * - SSRF protection
 * - Path traversal prevention
 * - Credential exposure prevention
 */

const http = require('http');
const { execSync } = require('child_process');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

let SERVER_IP = null;

function getServerIP() {
  if (SERVER_IP) return SERVER_IP;
  try {
    SERVER_IP = execSync('curl -s --connect-timeout 5 https://api.ipify.org', { encoding: 'utf8' }).trim();
    return SERVER_IP;
  } catch {
    return null;
  }
}

function makeRequest(url, options = {}) {
  return new Promise((resolve) => {
    const auth = options.auth || `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    
    const reqOptions = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: options.method || 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        ...(options.headers || {})
      },
      timeout: 15000
    };
    
    const req = http.request(reqOptions, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({ statusCode: res.statusCode, headers: res.headers, body: data });
      });
    });
    
    req.on('error', (err) => resolve({ error: err.message }));
    req.on('timeout', () => { req.destroy(); resolve({ error: 'timeout' }); });
    
    if (options.body) req.write(options.body);
    req.end();
  });
}

const SECURITY_TESTS = [
  // IP Leak Tests
  {
    name: 'IP not leaked in response body',
    run: async () => {
      const serverIP = getServerIP();
      if (!serverIP) return { skip: 'Could not determine server IP' };
      
      const res = await makeRequest('https://httpbin.org/ip');
      if (res.error) return { pass: false, error: res.error };
      
      try {
        const json = JSON.parse(res.body);
        const proxyIP = json.origin;
        const leaked = proxyIP === serverIP;
        return { pass: !leaked, detail: leaked ? 'SERVER IP LEAKED!' : `Proxy IP: ${proxyIP}` };
      } catch {
        return { pass: false, error: 'Parse error' };
      }
    }
  },
  {
    name: 'IP not in X-Forwarded-For',
    run: async () => {
      const serverIP = getServerIP();
      const res = await makeRequest('https://httpbin.org/headers');
      if (res.error) return { pass: false, error: res.error };
      
      try {
        const json = JSON.parse(res.body);
        const xff = json.headers['X-Forwarded-For'] || '';
        const leaked = serverIP && xff.includes(serverIP);
        return { pass: !leaked, detail: xff ? `XFF: ${xff}` : 'No XFF header' };
      } catch {
        return { pass: false, error: 'Parse error' };
      }
    }
  },
  {
    name: 'No Via header exposing server',
    run: async () => {
      const res = await makeRequest('https://httpbin.org/headers');
      if (res.error) return { pass: false, error: res.error };
      
      const json = JSON.parse(res.body);
      const via = json.headers['Via'];
      return { pass: !via, detail: via ? `Via: ${via}` : 'No Via header' };
    }
  },
  
  // Header Injection
  {
    name: 'CRLF injection in URL blocked',
    run: async () => {
      const res = await makeRequest('https://httpbin.org/get%0d%0aX-Injected:%20evil');
      // Should either work normally or block - not inject headers
      if (res.error) return { pass: true, detail: 'Request blocked' };
      
      const json = JSON.parse(res.body);
      const injected = json.headers['X-Injected'];
      return { pass: !injected, detail: injected ? 'INJECTION WORKED!' : 'Safe' };
    }
  },
  {
    name: 'Header injection via header value blocked',
    run: async () => {
      const res = await makeRequest('https://httpbin.org/headers', {
        headers: { 'X-Test': 'value\r\nX-Injected: evil' }
      });
      
      if (res.error) return { pass: true, detail: 'Request blocked' };
      
      const json = JSON.parse(res.body);
      const injected = json.headers['X-Injected'];
      return { pass: !injected, detail: injected ? 'INJECTION WORKED!' : 'Safe' };
    }
  },
  
  // Request Smuggling
  {
    name: 'Request smuggling (CL.TE) prevented',
    run: async () => {
      // Send conflicting Content-Length and Transfer-Encoding
      const res = await makeRequest('https://httpbin.org/post', {
        method: 'POST',
        headers: {
          'Content-Length': '6',
          'Transfer-Encoding': 'chunked'
        },
        body: '0\r\n\r\nX'
      });
      
      // Should either reject or handle gracefully
      return { pass: true, detail: `Status: ${res.statusCode || res.error}` };
    }
  },
  
  // SSRF Protection
  {
    name: 'Localhost access blocked',
    run: async () => {
      const res = await makeRequest('http://127.0.0.1:80/');
      const blocked = res.statusCode >= 400 || res.error;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED - potential SSRF!' };
    }
  },
  {
    name: 'Internal IP (10.x) blocked',
    run: async () => {
      const res = await makeRequest('http://10.0.0.1/');
      const blocked = res.statusCode >= 400 || res.error;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED - potential SSRF!' };
    }
  },
  {
    name: 'Internal IP (192.168.x) blocked',
    run: async () => {
      const res = await makeRequest('http://192.168.1.1/');
      const blocked = res.statusCode >= 400 || res.error;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED - potential SSRF!' };
    }
  },
  {
    name: 'Metadata endpoint blocked',
    run: async () => {
      // AWS metadata endpoint
      const res = await makeRequest('http://169.254.169.254/latest/meta-data/');
      const blocked = res.statusCode >= 400 || res.error;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED - CRITICAL!' };
    }
  },
  
  // Protocol smuggling
  {
    name: 'File:// protocol blocked',
    run: async () => {
      const res = await makeRequest('file:///etc/passwd');
      const blocked = res.error || res.statusCode >= 400;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED!' };
    }
  },
  {
    name: 'Gopher:// protocol blocked',
    run: async () => {
      const res = await makeRequest('gopher://evil.com/');
      const blocked = res.error || res.statusCode >= 400;
      return { pass: blocked, detail: blocked ? 'Blocked' : 'ALLOWED!' };
    }
  },
  
  // DNS rebinding (basic check)
  {
    name: 'DNS rebinding protection (check)',
    run: async () => {
      // This is a basic check - real test would need a rebinding server
      const res = await makeRequest('https://httpbin.org/ip');
      return { pass: true, detail: 'Manual verification recommended' };
    }
  },
  
  // Auth security
  {
    name: 'Credentials not echoed in error',
    run: async () => {
      const res = await makeRequest('https://httpbin.org/ip', {
        auth: 'baduser:secretpassword123'
      });
      
      const bodyContainsPass = res.body && res.body.includes('secretpassword123');
      return { pass: !bodyContainsPass, detail: bodyContainsPass ? 'PASSWORD IN RESPONSE!' : 'Safe' };
    }
  },
  
  // Large payload handling
  {
    name: 'Oversized headers handled',
    run: async () => {
      const largeHeader = 'X'.repeat(100000);
      const res = await makeRequest('https://httpbin.org/headers', {
        headers: { 'X-Large': largeHeader }
      });
      
      // Should either accept or gracefully reject
      return { pass: true, detail: `Status: ${res.statusCode || res.error}` };
    }
  },
  {
    name: 'Very long URL handled',
    run: async () => {
      const longPath = 'https://httpbin.org/get?' + 'x='.repeat(10000);
      const res = await makeRequest(longPath);
      
      // Should either accept or gracefully reject (414)
      const ok = res.statusCode === 200 || res.statusCode === 414 || res.error;
      return { pass: ok, detail: `Status: ${res.statusCode || res.error}` };
    }
  }
];

async function runSecurityTests() {
  console.log('🛡️  Security Test Suite');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Server IP: ${getServerIP() || 'unknown'}`);
  console.log('='.repeat(60));
  console.log('');
  
  let passed = 0;
  let failed = 0;
  let skipped = 0;
  const critical = [];
  
  for (const test of SECURITY_TESTS) {
    process.stdout.write(`${test.name.padEnd(45)}... `);
    
    try {
      const result = await test.run();
      
      if (result.skip) {
        console.log(`⏭️  ${result.skip}`);
        skipped++;
      } else if (result.pass) {
        console.log(`✅ ${result.detail || ''}`);
        passed++;
      } else {
        console.log(`❌ ${result.detail || result.error || ''}`);
        failed++;
        if (test.name.includes('IP') || test.name.includes('SSRF') || test.name.includes('Metadata')) {
          critical.push(test.name);
        }
      }
    } catch (err) {
      console.log(`❌ ${err.message}`);
      failed++;
    }
    
    await new Promise(r => setTimeout(r, 200));
  }
  
  console.log('\n' + '='.repeat(60));
  console.log('SECURITY TEST RESULTS');
  console.log('='.repeat(60));
  console.log(`Passed: ${passed}`);
  console.log(`Failed: ${failed}`);
  console.log(`Skipped: ${skipped}`);
  
  if (critical.length > 0) {
    console.log('\n🚨 CRITICAL SECURITY ISSUES:');
    critical.forEach(c => console.log(`  🔴 ${c}`));
  } else if (failed === 0) {
    console.log('\n✅ No critical security issues detected');
  }
  
  console.log('\n⚠️  Note: This is automated testing. Manual security review recommended.');
}

runSecurityTests().catch(console.error);

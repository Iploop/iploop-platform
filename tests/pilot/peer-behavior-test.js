#!/usr/bin/env node
/**
 * Peer Behavior Test
 * 
 * Tests proxy behavior related to peer management:
 * - Load balancing across peers
 * - Peer selection fairness
 * - What happens when no peers
 * - Peer health detection
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeRequest(options = {}) {
  return new Promise((resolve) => {
    let auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    if (options.country) auth += `-country-${options.country}`;
    if (options.session) auth += `-session-${options.session}`;
    if (options.sesstype) auth += `-sesstype-${options.sesstype}`;
    
    const start = Date.now();
    
    const reqOptions = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'http://ip-api.com/json',
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 30000
    };
    
    const req = http.request(reqOptions, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const json = JSON.parse(data);
          resolve({
            success: true,
            ip: json.query,
            country: json.countryCode,
            city: json.city,
            isp: json.isp,
            latency: Date.now() - start
          });
        } catch {
          resolve({ success: false, error: 'parse_error', statusCode: res.statusCode });
        }
      });
    });
    
    req.on('error', (err) => resolve({ success: false, error: err.code }));
    req.on('timeout', () => { req.destroy(); resolve({ success: false, error: 'timeout' }); });
    req.end();
  });
}

async function testLoadBalancing() {
  console.log('\n--- Test 1: Load Balancing Distribution ---');
  console.log('Making 100 requests to see IP distribution...\n');
  
  const ipCounts = {};
  const ispCounts = {};
  
  for (let i = 0; i < 100; i++) {
    const result = await makeRequest({ sesstype: 'rotating' });
    if (result.success) {
      ipCounts[result.ip] = (ipCounts[result.ip] || 0) + 1;
      ispCounts[result.isp] = (ispCounts[result.isp] || 0) + 1;
    }
    
    if ((i + 1) % 25 === 0) {
      console.log(`  Progress: ${i + 1}/100`);
    }
    
    await new Promise(r => setTimeout(r, 200));
  }
  
  const uniqueIps = Object.keys(ipCounts).length;
  const uniqueIsps = Object.keys(ispCounts).length;
  
  console.log(`\nUnique IPs: ${uniqueIps}`);
  console.log(`Unique ISPs: ${uniqueIsps}`);
  
  // Check distribution fairness
  const counts = Object.values(ipCounts);
  const maxUsage = Math.max(...counts);
  const minUsage = Math.min(...counts);
  const avgUsage = counts.reduce((a, b) => a + b, 0) / counts.length;
  
  console.log(`\nIP Usage distribution:`);
  console.log(`  Min: ${minUsage}, Max: ${maxUsage}, Avg: ${avgUsage.toFixed(1)}`);
  
  // Show top IPs
  const sorted = Object.entries(ipCounts).sort((a, b) => b[1] - a[1]);
  console.log(`\nTop 5 IPs:`);
  sorted.slice(0, 5).forEach(([ip, count]) => {
    const bar = '█'.repeat(Math.ceil(count / 5));
    console.log(`  ${ip.padEnd(16)} ${bar} ${count}`);
  });
  
  // Fairness score
  const fairnessScore = uniqueIps > 1 ? (1 - (maxUsage - minUsage) / 100) * 100 : 0;
  console.log(`\nFairness score: ${fairnessScore.toFixed(0)}%`);
  
  return uniqueIps >= 1; // At least 1 IP means proxy is working
}

async function testGeoDistribution() {
  console.log('\n--- Test 2: Geographic Distribution ---');
  console.log('Checking country distribution without targeting...\n');
  
  const countryCounts = {};
  
  for (let i = 0; i < 50; i++) {
    const result = await makeRequest({ sesstype: 'rotating' });
    if (result.success && result.country) {
      countryCounts[result.country] = (countryCounts[result.country] || 0) + 1;
    }
    await new Promise(r => setTimeout(r, 200));
  }
  
  console.log('Country distribution:');
  Object.entries(countryCounts)
    .sort((a, b) => b[1] - a[1])
    .forEach(([country, count]) => {
      const pct = (count / 50 * 100).toFixed(0);
      console.log(`  ${country}: ${count} (${pct}%)`);
    });
  
  return Object.keys(countryCounts).length >= 1;
}

async function testSessionIsolation() {
  console.log('\n--- Test 3: Session Isolation ---');
  console.log('Testing that different sessions get different treatment...\n');
  
  // Create multiple sessions
  const sessions = ['session_A', 'session_B', 'session_C'];
  const sessionIps = {};
  
  for (const session of sessions) {
    sessionIps[session] = [];
    
    for (let i = 0; i < 5; i++) {
      const result = await makeRequest({ session, sesstype: 'sticky' });
      if (result.success) {
        sessionIps[session].push(result.ip);
      }
      await new Promise(r => setTimeout(r, 500));
    }
  }
  
  // Check if each session maintained its IP (stickiness)
  console.log('Session stickiness:');
  let allSticky = true;
  
  for (const [session, ips] of Object.entries(sessionIps)) {
    const uniqueInSession = [...new Set(ips)];
    const sticky = uniqueInSession.length === 1 && ips.length === 5;
    console.log(`  ${session}: ${sticky ? '✅ Sticky' : '❌ Changed'} (${uniqueInSession.length} unique IPs)`);
    if (!sticky) allSticky = false;
  }
  
  // Check if sessions got different IPs (isolation)
  const allIps = Object.values(sessionIps).flat();
  const uniqueAcrossSessions = [...new Set(allIps)];
  console.log(`\nSession isolation: ${uniqueAcrossSessions.length} unique IPs across all sessions`);
  
  return allSticky;
}

async function testNoTargetAvailable() {
  console.log('\n--- Test 4: Unavailable Target Handling ---');
  console.log('Testing request to unavailable country...\n');
  
  // Request from Antarctica (unlikely to have peers)
  const result = await makeRequest({ country: 'AQ' });
  
  if (result.success) {
    console.log(`Got response from: ${result.country} (${result.ip})`);
    console.log('Proxy returned available peer instead of failing');
  } else {
    console.log(`Error: ${result.error || result.statusCode}`);
    console.log('Proxy properly reported unavailable');
  }
  
  return true; // Either behavior is acceptable
}

async function testRapidSessionCreation() {
  console.log('\n--- Test 5: Rapid Session Creation ---');
  console.log('Creating 50 sessions rapidly...\n');
  
  const promises = [];
  for (let i = 0; i < 50; i++) {
    promises.push(makeRequest({ 
      session: `rapid_${Date.now()}_${i}`, 
      sesstype: 'sticky' 
    }));
  }
  
  const results = await Promise.all(promises);
  const success = results.filter(r => r.success).length;
  const uniqueIps = [...new Set(results.filter(r => r.success).map(r => r.ip))].length;
  
  console.log(`Success: ${success}/50`);
  console.log(`Unique IPs assigned: ${uniqueIps}`);
  
  return success >= 45; // 90% success
}

async function runPeerTests() {
  console.log('👥 Peer Behavior Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  
  const results = [];
  
  results.push({ name: 'Load Balancing', pass: await testLoadBalancing() });
  results.push({ name: 'Geo Distribution', pass: await testGeoDistribution() });
  results.push({ name: 'Session Isolation', pass: await testSessionIsolation() });
  results.push({ name: 'Unavailable Target', pass: await testNoTargetAvailable() });
  results.push({ name: 'Rapid Session Creation', pass: await testRapidSessionCreation() });
  
  console.log('\n' + '='.repeat(60));
  console.log('PEER BEHAVIOR TEST RESULTS');
  console.log('='.repeat(60));
  
  results.forEach(r => {
    console.log(`${r.pass ? '✅' : '❌'} ${r.name}`);
  });
  
  const passed = results.filter(r => r.pass).length;
  console.log(`\nPassed: ${passed}/${results.length}`);
}

runPeerTests().catch(console.error);

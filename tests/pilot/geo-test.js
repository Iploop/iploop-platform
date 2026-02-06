#!/usr/bin/env node
/**
 * IPLoop Geographic Targeting Test
 * 
 * Verifies that proxy requests are routed through correct countries
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

const COUNTRIES_TO_TEST = ['US', 'DE', 'GB', 'FR', 'JP', 'AU', 'BR', 'IN'];

function makeRequest(targetCountry) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}-country-${targetCountry}`;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'http://ip-api.com/json',
      headers: {
        'Host': 'ip-api.com',
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
          resolve({
            requested: targetCountry,
            actual: json.countryCode,
            ip: json.query,
            city: json.city,
            isp: json.isp,
            match: json.countryCode === targetCountry
          });
        } catch (e) {
          reject({ error: 'Parse error', data });
        }
      });
    });
    
    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy();
      reject({ error: 'Timeout' });
    });
    
    req.end();
  });
}

async function runGeoTest() {
  console.log('🗺️  IPLoop Geographic Targeting Test');
  console.log('='.repeat(50));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(50));
  console.log('');
  
  const results = [];
  
  for (const country of COUNTRIES_TO_TEST) {
    process.stdout.write(`Testing ${country}... `);
    
    try {
      const result = await makeRequest(country);
      results.push(result);
      
      if (result.match) {
        console.log(`✅ ${result.actual} (${result.city}, ${result.isp})`);
      } else {
        console.log(`❌ Got ${result.actual} instead (${result.ip})`);
      }
    } catch (err) {
      results.push({ requested: country, error: err.error || err.message });
      console.log(`❌ Error: ${err.error || err.message}`);
    }
    
    // Small delay between requests
    await new Promise(r => setTimeout(r, 1000));
  }
  
  // Summary
  console.log('\n' + '='.repeat(50));
  console.log('SUMMARY');
  console.log('='.repeat(50));
  
  const passed = results.filter(r => r.match).length;
  const failed = results.filter(r => !r.match && !r.error).length;
  const errors = results.filter(r => r.error).length;
  
  console.log(`Passed: ${passed}/${COUNTRIES_TO_TEST.length}`);
  console.log(`Failed (wrong country): ${failed}`);
  console.log(`Errors: ${errors}`);
  
  if (passed === COUNTRIES_TO_TEST.length) {
    console.log('\n✅ All geographic targeting tests passed!');
  } else {
    console.log('\n⚠️  Some geographic targeting tests failed');
  }
}

runGeoTest().catch(console.error);

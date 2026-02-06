/**
 * Test Wrapper - Captures test output and saves results
 * 
 * Usage: node lib/test-wrapper.js <test-script> [args...]
 */

const { spawn } = require('child_process');
const path = require('path');
const { saveResult } = require('./reporter');

const args = process.argv.slice(2);
if (args.length === 0) {
  console.error('Usage: node lib/test-wrapper.js <test-script> [args...]');
  process.exit(1);
}

const scriptPath = args[0];
const scriptArgs = args.slice(1);
const testName = path.basename(scriptPath, '.js');

const startTime = Date.now();
let output = '';
let passed = true;
const metrics = {};

console.log(`\n🧪 Running test: ${testName}`);
console.log('─'.repeat(50));

const proc = spawn('node', [scriptPath, ...scriptArgs], {
  cwd: path.join(__dirname, '..'),
  env: process.env,
  stdio: ['inherit', 'pipe', 'pipe']
});

proc.stdout.on('data', (data) => {
  const text = data.toString();
  output += text;
  process.stdout.write(text);
  
  // Parse metrics from output
  parseMetrics(text, metrics);
});

proc.stderr.on('data', (data) => {
  const text = data.toString();
  output += text;
  process.stderr.write(text);
  
  // Check for failures
  if (text.includes('FAILED') || text.includes('Error:') || text.includes('FAIL')) {
    passed = false;
  }
});

proc.on('close', (code) => {
  const duration = Date.now() - startTime;
  
  if (code !== 0) {
    passed = false;
  }
  
  // Check output for pass/fail indicators
  if (output.includes('All tests passed') || output.includes('✓')) {
    passed = true;
  }
  if (output.includes('FAILED') || output.includes('❌')) {
    passed = false;
  }
  
  // Save result
  const result = {
    passed,
    duration,
    exitCode: code,
    summary: metrics,
    output: output.slice(-10000) // Keep last 10K chars
  };
  
  saveResult(testName, result);
  
  console.log('\n' + '─'.repeat(50));
  console.log(`${passed ? '✅' : '❌'} Test ${testName} ${passed ? 'PASSED' : 'FAILED'} in ${duration}ms`);
  
  process.exit(code);
});

/**
 * Parse metrics from test output
 */
function parseMetrics(text, metrics) {
  // Common patterns to extract metrics
  const patterns = [
    // Success rate: 98.5%
    /success\s*rate[:\s]+([0-9.]+)%/i,
    // Total requests: 1234
    /total\s*requests?[:\s]+([0-9,]+)/i,
    // Failed: 12
    /failed[:\s]+([0-9,]+)/i,
    // Avg latency: 150ms
    /avg\s*latency[:\s]+([0-9.]+)\s*ms/i,
    // P99: 500ms
    /p99[:\s]+([0-9.]+)\s*ms/i,
    // Throughput: 100 req/s
    /throughput[:\s]+([0-9.]+)\s*req/i,
    // Errors: 5
    /errors?[:\s]+([0-9,]+)/i,
    // IPs observed: 15
    /ips?\s*observed[:\s]+([0-9,]+)/i,
    // Unique IPs: 15
    /unique\s*ips?[:\s]+([0-9,]+)/i,
  ];
  
  const names = [
    'successRate',
    'totalRequests',
    'failed',
    'avgLatency',
    'p99Latency',
    'throughput',
    'errors',
    'ipsObserved',
    'uniqueIps'
  ];
  
  patterns.forEach((pattern, i) => {
    const match = text.match(pattern);
    if (match) {
      const value = parseFloat(match[1].replace(/,/g, ''));
      metrics[names[i]] = value;
    }
  });
}

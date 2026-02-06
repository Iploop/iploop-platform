#!/usr/bin/env node
/**
 * IPLoop Autonomous Test Scheduler
 * Runs tests automatically on defined schedules
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const TESTS_DIR = __dirname;
const RESULTS_DIR = path.join(__dirname, 'results');
const STATE_FILE = path.join(RESULTS_DIR, 'scheduler-state.json');
const LIVE_STATE_FILE = path.join(RESULTS_DIR, 'live-state.json');
const LIVE_OUTPUT_FILE = path.join(RESULTS_DIR, 'live-output.log');

// Test suites with schedules (in minutes)
const SCHEDULES = {
  quick: {
    interval: 30,  // Every 30 minutes
    tests: ['stress-test.js --profile=smoke', 'leak-test.js', 'protocol-test.js'],
    description: 'Quick health check'
  },
  security: {
    interval: 120,  // Every 2 hours
    tests: ['security-test.js', 'auth-test.js', 'leak-test.js'],
    description: 'Security audit'
  },
  feature: {
    interval: 60,  // Every hour
    tests: ['geo-test.js', 'rotation-test.js', 'sticky-stress.js'],
    description: 'Feature validation'
  },
  performance: {
    interval: 180,  // Every 3 hours
    tests: ['bandwidth-test.js', 'latency-test.js', 'stress-test.js --profile=ramp'],
    description: 'Performance benchmark'
  },
  full: {
    interval: 360,  // Every 6 hours
    tests: [
      'stress-test.js --profile=smoke',
      'geo-test.js', 'leak-test.js', 'security-test.js', 'auth-test.js',
      'protocol-test.js', 'bandwidth-test.js', 'failure-test.js',
      'rotation-test.js', 'sticky-stress.js', 'latency-test.js',
      'connection-limits.js', 'concurrency-edge.js', 'peer-behavior-test.js'
    ],
    description: 'Full test suite'
  }
};

// State management
function loadState() {
  try {
    if (fs.existsSync(STATE_FILE)) {
      return JSON.parse(fs.readFileSync(STATE_FILE, 'utf8'));
    }
  } catch (e) {}
  return { lastRuns: {}, currentRun: null };
}

function saveState(state) {
  fs.mkdirSync(RESULTS_DIR, { recursive: true });
  fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2));
}

function updateLiveState(liveState) {
  fs.writeFileSync(LIVE_STATE_FILE, JSON.stringify(liveState, null, 2));
}

function appendLiveOutput(line) {
  fs.appendFileSync(LIVE_OUTPUT_FILE, line + '\n');
}

function clearLiveOutput() {
  fs.writeFileSync(LIVE_OUTPUT_FILE, '');
}

// Run a single test
function runTest(testCmd, liveState) {
  return new Promise((resolve) => {
    const [script, ...args] = testCmd.split(' ');
    const startTime = Date.now();
    
    console.log(`  🧪 Running: ${testCmd}`);
    appendLiveOutput(`\n${'─'.repeat(50)}`);
    appendLiveOutput(`🧪 Running: ${testCmd}`);
    appendLiveOutput(`${'─'.repeat(50)}`);
    
    // Update live state
    liveState.currentTest = script;
    liveState.startTime = startTime;
    updateLiveState(liveState);
    
    const proc = spawn('node', ['lib/test-wrapper.js', script, ...args], {
      cwd: TESTS_DIR,
      env: {
        ...process.env,
        PROXY_HOST: process.env.PROXY_HOST || 'localhost',
        PROXY_PORT: process.env.PROXY_PORT || '8080',
        PROXY_USER: process.env.PROXY_USER || 'test_customer',
        PROXY_PASS: process.env.PROXY_PASS || 'test_api_key'
      },
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let output = '';
    proc.stdout.on('data', d => {
      output += d;
      appendLiveOutput(d.toString().trim());
    });
    proc.stderr.on('data', d => {
      output += d;
      appendLiveOutput(d.toString().trim());
    });

    proc.on('close', (code) => {
      const duration = Date.now() - startTime;
      const passed = code === 0 && !output.includes('FAILED');
      const resultLine = `${passed ? '✅' : '❌'} ${script} - ${(duration / 1000).toFixed(1)}s`;
      console.log(`  ${resultLine}`);
      appendLiveOutput(resultLine);
      
      liveState.lastTest = { name: script, passed, duration };
      liveState.progress++;
      updateLiveState(liveState);
      
      resolve({ script, passed, duration, code });
    });

    proc.on('error', (err) => {
      console.log(`  ❌ ${script} - Error: ${err.message}`);
      appendLiveOutput(`❌ ${script} - Error: ${err.message}`);
      resolve({ script, passed: false, duration: 0, error: err.message });
    });
  });
}

// Run a test suite
async function runSuite(suiteName) {
  const suite = SCHEDULES[suiteName];
  if (!suite) {
    console.error(`Unknown suite: ${suiteName}`);
    return;
  }

  // Clear and initialize live output
  clearLiveOutput();
  
  const liveState = {
    running: true,
    suite: suiteName,
    description: suite.description,
    total: suite.tests.length,
    progress: 0,
    currentTest: null,
    startTime: null,
    suiteStartTime: Date.now(),
    queue: [...suite.tests],
    lastTest: null
  };
  updateLiveState(liveState);

  console.log(`\n${'═'.repeat(60)}`);
  console.log(`📋 Running ${suiteName.toUpperCase()} suite: ${suite.description}`);
  console.log(`${'═'.repeat(60)}\n`);
  
  appendLiveOutput(`${'═'.repeat(50)}`);
  appendLiveOutput(`📋 ${suiteName.toUpperCase()} SUITE: ${suite.description}`);
  appendLiveOutput(`Total tests: ${suite.tests.length}`);
  appendLiveOutput(`${'═'.repeat(50)}`);

  const results = [];
  for (let i = 0; i < suite.tests.length; i++) {
    const test = suite.tests[i];
    liveState.queue = suite.tests.slice(i + 1);
    updateLiveState(liveState);
    
    const result = await runTest(test, liveState);
    results.push(result);
    // Small delay between tests
    await new Promise(r => setTimeout(r, 1000));
  }

  const passed = results.filter(r => r.passed).length;
  const failed = results.length - passed;
  
  console.log(`\n${'─'.repeat(60)}`);
  console.log(`📊 Suite complete: ${passed}/${results.length} passed, ${failed} failed`);
  console.log(`${'─'.repeat(60)}\n`);
  
  appendLiveOutput(`\n${'═'.repeat(50)}`);
  appendLiveOutput(`📊 SUITE COMPLETE: ${passed}/${results.length} passed, ${failed} failed`);
  appendLiveOutput(`${'═'.repeat(50)}`);

  // Update live state to idle
  liveState.running = false;
  liveState.currentTest = null;
  liveState.queue = [];
  updateLiveState(liveState);

  return { suiteName, passed, failed, total: results.length, results };
}

// Check which suites need to run
function getSuitesDue(state) {
  const now = Date.now();
  const due = [];

  for (const [name, config] of Object.entries(SCHEDULES)) {
    const lastRun = state.lastRuns[name] || 0;
    const intervalMs = config.interval * 60 * 1000;
    
    if (now - lastRun >= intervalMs) {
      due.push({ name, config, overdue: now - lastRun - intervalMs });
    }
  }

  // Sort by most overdue first
  return due.sort((a, b) => b.overdue - a.overdue);
}

// Main scheduler loop
async function runScheduler() {
  console.log(`
╔══════════════════════════════════════════════════════════════════╗
║              IPLoop Autonomous Test Scheduler                    ║
╚══════════════════════════════════════════════════════════════════╝
`);

  console.log('Schedules:');
  for (const [name, config] of Object.entries(SCHEDULES)) {
    console.log(`  • ${name}: every ${config.interval}min (${config.tests.length} tests)`);
  }
  console.log('\nStarting scheduler loop...\n');

  while (true) {
    const state = loadState();
    const due = getSuitesDue(state);

    if (due.length > 0) {
      // Run the most overdue suite
      const { name } = due[0];
      state.currentRun = { suite: name, startedAt: Date.now() };
      saveState(state);

      await runSuite(name);

      state.lastRuns[name] = Date.now();
      state.currentRun = null;
      saveState(state);
    } else {
      // Find next scheduled run
      const nextRuns = Object.entries(SCHEDULES).map(([name, config]) => {
        const lastRun = state.lastRuns[name] || 0;
        const nextRun = lastRun + (config.interval * 60 * 1000);
        return { name, nextRun, inMinutes: Math.ceil((nextRun - Date.now()) / 60000) };
      }).sort((a, b) => a.nextRun - b.nextRun);

      const next = nextRuns[0];
      console.log(`⏰ Next run: ${next.name} in ${next.inMinutes} minutes`);
    }

    // Check every minute
    await new Promise(r => setTimeout(r, 60000));
  }
}

// CLI
const args = process.argv.slice(2);
const cmd = args[0];

if (cmd === 'run') {
  // Run a specific suite immediately
  const suite = args[1] || 'quick';
  runSuite(suite).then(() => process.exit(0));
} else if (cmd === 'status') {
  // Show status
  const state = loadState();
  console.log('Scheduler State:');
  console.log(JSON.stringify(state, null, 2));
  
  console.log('\nNext runs:');
  for (const [name, config] of Object.entries(SCHEDULES)) {
    const lastRun = state.lastRuns[name] || 0;
    const nextRun = lastRun + (config.interval * 60 * 1000);
    const inMinutes = Math.ceil((nextRun - Date.now()) / 60000);
    console.log(`  ${name}: ${inMinutes > 0 ? `in ${inMinutes}m` : 'now'}`);
  }
} else if (cmd === 'daemon' || !cmd) {
  // Run as daemon
  runScheduler().catch(console.error);
} else {
  console.log(`
Usage: node scheduler.js [command]

Commands:
  daemon      Run scheduler daemon (default)
  run <suite> Run a specific suite immediately
  status      Show scheduler status

Suites: ${Object.keys(SCHEDULES).join(', ')}
`);
}

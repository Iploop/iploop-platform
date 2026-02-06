#!/usr/bin/env node
/**
 * IPLoop Test Dashboard Server
 * Simple standalone server for test results visualization
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');

const PORT = process.env.PORT || 8888;
const RESULTS_DIR = path.join(__dirname, '..', 'results');
const TESTS_DIR = path.join(__dirname, '..');

// Ensure results directory exists
if (!fs.existsSync(RESULTS_DIR)) {
  fs.mkdirSync(RESULTS_DIR, { recursive: true });
}

// Test command mappings
const TEST_COMMANDS = {
  'stress-test': ['node', 'lib/test-wrapper.js', 'stress-test.js', '--profile=smoke'],
  'leak-test': ['node', 'lib/test-wrapper.js', 'leak-test.js'],
  'protocol-test': ['node', 'lib/test-wrapper.js', 'protocol-test.js'],
  'geo-test': ['node', 'lib/test-wrapper.js', 'geo-test.js'],
  'sticky-stress': ['node', 'lib/test-wrapper.js', 'sticky-stress.js'],
  'rotation-test': ['node', 'lib/test-wrapper.js', 'rotation-test.js'],
  'peer-behavior-test': ['node', 'lib/test-wrapper.js', 'peer-behavior-test.js'],
  'security-test': ['node', 'lib/test-wrapper.js', 'security-test.js'],
  'auth-test': ['node', 'lib/test-wrapper.js', 'auth-test.js'],
  'bandwidth-test': ['node', 'lib/test-wrapper.js', 'bandwidth-test.js'],
  'failure-test': ['node', 'lib/test-wrapper.js', 'failure-test.js'],
  'latency-test': ['node', 'lib/test-wrapper.js', 'latency-test.js'],
  'connection-limits': ['node', 'lib/test-wrapper.js', 'connection-limits.js'],
  'concurrency-edge': ['node', 'lib/test-wrapper.js', 'concurrency-edge.js'],
  'stability-test': ['node', 'lib/test-wrapper.js', 'stability-test.js'],
  'scenario-price-scrape': ['node', 'lib/test-wrapper.js', 'scenario-price-scrape.js'],
};

// Schedule definitions
const SCHEDULES = {
  quick: { interval: 30, tests: 3, description: 'Quick health check' },
  security: { interval: 120, tests: 3, description: 'Security audit' },
  feature: { interval: 60, tests: 3, description: 'Feature validation' },
  performance: { interval: 180, tests: 3, description: 'Performance benchmark' },
  full: { interval: 360, tests: 14, description: 'Full test suite' }
};

// Get all results
function getResults() {
  const latest = {};
  const history = {};

  // Read latest.json
  const latestPath = path.join(RESULTS_DIR, 'latest.json');
  if (fs.existsSync(latestPath)) {
    try {
      Object.assign(latest, JSON.parse(fs.readFileSync(latestPath, 'utf8')));
    } catch (e) {}
  }

  // Get history for each test
  const files = fs.readdirSync(RESULTS_DIR)
    .filter(f => f.endsWith('.json') && f !== 'latest.json' && f !== 'scheduler-state.json')
    .sort()
    .reverse();

  for (const file of files) {
    try {
      const data = JSON.parse(fs.readFileSync(path.join(RESULTS_DIR, file), 'utf8'));
      const testName = data.testName;
      if (!history[testName]) history[testName] = [];
      if (history[testName].length < 50) {
        history[testName].push(data);
      }
    } catch (e) {}
  }

  // Get scheduler state
  const schedulerPath = path.join(RESULTS_DIR, 'scheduler-state.json');
  let schedule = {};
  if (fs.existsSync(schedulerPath)) {
    try {
      const state = JSON.parse(fs.readFileSync(schedulerPath, 'utf8'));
      for (const [name, config] of Object.entries(SCHEDULES)) {
        const lastRun = state.lastRuns?.[name] || 0;
        const nextRun = lastRun + (config.interval * 60 * 1000);
        schedule[name] = {
          ...config,
          lastRun,
          nextRun,
          inMinutes: Math.max(0, Math.ceil((nextRun - Date.now()) / 60000))
        };
      }
    } catch (e) {}
  }

  return { latest, history, schedule };
}

// Run a test and stream output
function runTest(testName, res) {
  const cmd = TEST_COMMANDS[testName];
  if (!cmd) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end('Unknown test: ' + testName);
    return;
  }

  res.writeHead(200, {
    'Content-Type': 'text/plain; charset=utf-8',
    'Transfer-Encoding': 'chunked',
    'Cache-Control': 'no-cache'
  });

  const [bin, ...args] = cmd;
  const proc = spawn(bin, args, {
    cwd: TESTS_DIR,
    env: {
      ...process.env,
      PROXY_HOST: process.env.PROXY_HOST || 'localhost',
      PROXY_PORT: process.env.PROXY_PORT || '8080',
      PROXY_USER: process.env.PROXY_USER || 'test_customer',
      PROXY_PASS: process.env.PROXY_PASS || 'test_api_key'
    }
  });

  proc.stdout.on('data', data => res.write(data));
  proc.stderr.on('data', data => res.write(data));
  proc.on('close', code => {
    res.write(`\n\nTest finished with code: ${code}\n`);
    res.end();
  });
  proc.on('error', err => {
    res.write(`\nError: ${err.message}\n`);
    res.end();
  });
}

// Agent response generator
function generateAgentResponse(agentInfo, message, history) {
  const msgLower = message.toLowerCase();
  const { name, role, style } = agentInfo;
  
  // Context-aware responses based on role and message content
  const responses = {
    greeting: [
      `Hey there! ${name} here, your ${role}. What can I help you with?`,
      `Hi! Ready to assist you with anything ${role.toLowerCase()}-related.`,
      `Hello! I'm ${name}. Let's get to work - what do you need?`
    ],
    status: [
      `Everything's running smoothly on my end. Current priorities are well-organized and on track.`,
      `All systems nominal! I've been keeping busy with the usual ${role.toLowerCase()} tasks.`,
      `Status check: We're in good shape. Want me to dive into any specifics?`
    ],
    help: [
      `Of course! As your ${role}, I can help with anything in my domain. Just tell me what you need.`,
      `I'm here to help! What specific area should we focus on?`,
      `Absolutely. What's on your mind?`
    ],
    task: [
      `Got it. I'll prioritize this and keep you updated on progress.`,
      `Consider it done. I'll work on this and report back.`,
      `I'm on it. Expect updates soon.`
    ],
    question: [
      `That's a great question. Let me think about this from a ${style} perspective...`,
      `Interesting point. Based on my experience as ${role}, I'd say we should approach this carefully.`,
      `I've been thinking about this too. Here's my take...`
    ],
    default: [
      `Understood. I'll factor this into my work as ${role}.`,
      `Thanks for letting me know. I'll keep this in mind.`,
      `Got it. Is there anything specific you'd like me to focus on?`
    ]
  };
  
  // Determine response type based on message
  let responseType = 'default';
  if (msgLower.match(/^(hi|hello|hey|good morning|good afternoon)/)) {
    responseType = 'greeting';
  } else if (msgLower.match(/status|how.*going|update|what.*working/)) {
    responseType = 'status';
  } else if (msgLower.match(/help|can you|could you|please/)) {
    responseType = 'help';
  } else if (msgLower.match(/do|make|create|build|write|send|prepare|generate/)) {
    responseType = 'task';
  } else if (msgLower.match(/\?|what|why|how|when|where|which/)) {
    responseType = 'question';
  }
  
  const options = responses[responseType];
  return options[Math.floor(Math.random() * options.length)];
}

// HTTP server
const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  
  // CORS
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  
  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    return res.end();
  }

  // Get live state
  function getLiveState() {
    const livePath = path.join(RESULTS_DIR, 'live-state.json');
    if (fs.existsSync(livePath)) {
      try {
        const state = JSON.parse(fs.readFileSync(livePath, 'utf8'));
        // Get output lines if available
        const outputPath = path.join(RESULTS_DIR, 'live-output.log');
        if (fs.existsSync(outputPath)) {
          const output = fs.readFileSync(outputPath, 'utf8').split('\n').slice(-200);
          state.output = output;
        }
        return state;
      } catch (e) {}
    }
    return { running: false, output: [] };
  }

  // Routes
  if (url.pathname === '/app' || url.pathname === '/app.html') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(fs.readFileSync(path.join(__dirname, 'app.html')));
  }
  else if (url.pathname === '/' || url.pathname === '/index.html' || url.pathname === '/testing') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(fs.readFileSync(path.join(__dirname, 'index.html')));
  }
  else if (url.pathname === '/api/results') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(getResults()));
  }
  else if (url.pathname === '/api/live') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(getLiveState()));
  }
  else if (url.pathname === '/api/system') {
    // Get system info
    const os = require('os');
    const { execSync } = require('child_process');
    
    let cpu = 0, memory = 0, disk = 0;
    try {
      cpu = parseFloat(execSync("top -bn1 | grep 'Cpu(s)' | awk '{print $2}'", { encoding: 'utf8' })) || 0;
      const memInfo = execSync("free | grep Mem | awk '{print $3/$2 * 100}'", { encoding: 'utf8' });
      memory = parseFloat(memInfo) || 0;
      const diskInfo = execSync("df / | tail -1 | awk '{print $5}'", { encoding: 'utf8' });
      disk = parseFloat(diskInfo) || 0;
    } catch (e) {}

    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      hostname: os.hostname(),
      os: 'Ubuntu 24.04 LTS',
      kernel: os.release() + ' (' + os.arch() + ')',
      nodeVersion: process.version,
      cpu: cpu.toFixed(1),
      memory: memory.toFixed(1),
      disk: disk.toFixed(1),
      uptime: os.uptime()
    }));
  }
  else if (url.pathname === '/ultron.html') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(fs.readFileSync(path.join(__dirname, 'ultron.html')));
  }
  else if (url.pathname === '/ai-teams.html') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(fs.readFileSync(path.join(__dirname, 'ai-teams.html')));
  }
  else if (url.pathname === '/api/leads') {
    const dataPath = path.join(__dirname, 'data', 'leads.json');
    if (fs.existsSync(dataPath)) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(fs.readFileSync(dataPath, 'utf8'));
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ leads: [] }));
    }
  }
  else if (url.pathname === '/api/content') {
    const dataPath = path.join(__dirname, 'data', 'content.json');
    if (fs.existsSync(dataPath)) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(fs.readFileSync(dataPath, 'utf8'));
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ content: [] }));
    }
  }
  else if (url.pathname === '/api/projects') {
    const dataPath = path.join(__dirname, 'data', 'projects.json');
    if (fs.existsSync(dataPath)) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(fs.readFileSync(dataPath, 'utf8'));
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ projects: [] }));
    }
  }
  else if (url.pathname === '/api/agents') {
    const dataPath = path.join(__dirname, 'data', 'agents.json');
    if (fs.existsSync(dataPath)) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(fs.readFileSync(dataPath, 'utf8'));
    } else {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ agents: {} }));
    }
  }
  else if (url.pathname === '/api/finance') {
    // Try to get live data from kuchiku snapshot first
    const kuchikuPath = '/root/clawd-secure/memory/kuchiku-snapshot.json';
    const fallbackPath = path.join(__dirname, 'data', 'finance.json');
    
    try {
      let financeData = {};
      
      if (fs.existsSync(kuchikuPath)) {
        const kuchiku = JSON.parse(fs.readFileSync(kuchikuPath, 'utf8'));
        financeData = {
          lastUpdated: kuchiku.date,
          overview: {
            totalRevenue: kuchiku.bigmama?.income || 0,
            totalPaid: kuchiku.bigmama?.paid || 0,
            pending: kuchiku.bigmama?.pending || 0,
            weeklyEarnings: kuchiku.bigmama?.weekly || 0
          },
          partners: (kuchiku.raw_data?.partners_sdk || []).map(p => ({
            name: p.partner,
            nodes: p.count,
            status: 'active'
          })),
          infrastructure: {
            desktopOnline: kuchiku.desktop?.total || 0,
            windowsOnline: kuchiku.desktop?.win_online || 0,
            macOnline: kuchiku.desktop?.mac_online || 0,
            androidOnline: kuchiku.android_online || 0,
            androidNew: kuchiku.android_new || 0,
            sdkTotal: kuchiku.sdk_total || 0,
            bigmamaTotal: kuchiku.bigmama?.total || 0,
            bigmamaOnline: kuchiku.bigmama?.online || 0,
            winTop5: kuchiku.win_top5 || [],
            macTop5: kuchiku.mac_top5 || []
          }
        };
      } else if (fs.existsSync(fallbackPath)) {
        financeData = JSON.parse(fs.readFileSync(fallbackPath, 'utf8'));
      }
      
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(financeData));
    } catch (e) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: e.message }));
    }
  }
  else if (url.pathname === '/api/chat' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
      try {
        const { agent, message, history } = JSON.parse(body);
        
        // Agent personalities
        const agentPersonalities = {
          'ultron': { name: 'Ultron', role: 'Central Orchestrator', style: 'analytical and efficient' },
          'sales-lead': { name: 'Marcus', role: 'Sales Director', style: 'persuasive and goal-oriented' },
          'lead-researcher': { name: 'Alex', role: 'Lead Researcher', style: 'thorough and data-driven' },
          'email-specialist': { name: 'Emma', role: 'Email Specialist', style: 'creative and psychology-aware' },
          'marketing-lead': { name: 'Sophia', role: 'Marketing Director', style: 'creative and strategic' },
          'content-writer': { name: 'Noah', role: 'Content Writer', style: 'eloquent and SEO-savvy' },
          'social-manager': { name: 'Olivia', role: 'Social Media Manager', style: 'trendy and engaging' },
          'dev-lead': { name: 'Liam', role: 'Tech Lead', style: 'technical and pragmatic' },
          'frontend-dev': { name: 'Mia', role: 'Frontend Developer', style: 'UX-focused and detail-oriented' },
          'backend-dev': { name: 'Ethan', role: 'Backend Developer', style: 'performance-focused and secure' },
          'partner-lead': { name: 'Ava', role: 'Partner Relations', style: 'diplomatic and relationship-focused' },
          'support-agent': { name: 'James', role: 'Support Specialist', style: 'patient and solution-oriented' },
          'finance-lead': { name: 'William', role: 'Finance Director', style: 'meticulous and numbers-focused' },
          'invoice-agent': { name: 'Luna', role: 'Billing Specialist', style: 'organized and precise' }
        };
        
        const agentInfo = agentPersonalities[agent] || { name: 'Agent', role: 'Assistant', style: 'helpful' };
        
        // Generate contextual response based on message content
        let reply = generateAgentResponse(agentInfo, message, history);
        
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ reply, agent: agentInfo.name }));
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'Invalid request', details: e.message }));
      }
    });
  }
  else if (url.pathname === '/api/run' && req.method === 'POST') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try {
        const { test } = JSON.parse(body);
        runTest(test, res);
      } catch (e) {
        res.writeHead(400, { 'Content-Type': 'text/plain' });
        res.end('Invalid request');
      }
    });
  }
  else {
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('Not Found');
  }
});

server.listen(PORT, () => {
  console.log(`
╔══════════════════════════════════════════════════════════════════╗
║              IPLoop Test Dashboard                               ║
║              http://localhost:${PORT}                               ║
╚══════════════════════════════════════════════════════════════════╝
`);
});

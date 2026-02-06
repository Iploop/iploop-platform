/**
 * IPLoop Test Results Reporter
 * Saves test results as JSON for dashboard consumption
 */

const fs = require('fs');
const path = require('path');

const RESULTS_DIR = path.join(__dirname, '..', 'results');

// Ensure results directory exists
if (!fs.existsSync(RESULTS_DIR)) {
  fs.mkdirSync(RESULTS_DIR, { recursive: true });
}

/**
 * Save test result to JSON file
 */
function saveResult(testName, result) {
  const timestamp = new Date().toISOString();
  const filename = `${testName}-${timestamp.replace(/[:.]/g, '-')}.json`;
  const filepath = path.join(RESULTS_DIR, filename);
  
  const data = {
    testName,
    timestamp,
    duration: result.duration || 0,
    ...result
  };
  
  fs.writeFileSync(filepath, JSON.stringify(data, null, 2));
  
  // Also update latest.json for quick access
  updateLatest(testName, data);
  
  console.log(`\n📊 Results saved to: ${filename}`);
  return filepath;
}

/**
 * Update latest results index
 */
function updateLatest(testName, data) {
  const latestPath = path.join(RESULTS_DIR, 'latest.json');
  let latest = {};
  
  if (fs.existsSync(latestPath)) {
    try {
      latest = JSON.parse(fs.readFileSync(latestPath, 'utf8'));
    } catch (e) {
      latest = {};
    }
  }
  
  latest[testName] = {
    timestamp: data.timestamp,
    summary: data.summary || {},
    passed: data.passed !== undefined ? data.passed : true,
    duration: data.duration
  };
  
  fs.writeFileSync(latestPath, JSON.stringify(latest, null, 2));
}

/**
 * Get historical results for a test
 */
function getHistory(testName, limit = 20) {
  const files = fs.readdirSync(RESULTS_DIR)
    .filter(f => f.startsWith(testName + '-') && f.endsWith('.json'))
    .sort()
    .reverse()
    .slice(0, limit);
  
  return files.map(f => {
    try {
      return JSON.parse(fs.readFileSync(path.join(RESULTS_DIR, f), 'utf8'));
    } catch (e) {
      return null;
    }
  }).filter(Boolean);
}

/**
 * Get all test results
 */
function getAllResults() {
  const latestPath = path.join(RESULTS_DIR, 'latest.json');
  if (fs.existsSync(latestPath)) {
    return JSON.parse(fs.readFileSync(latestPath, 'utf8'));
  }
  return {};
}

/**
 * Calculate comparison between two results
 */
function compare(current, previous) {
  if (!previous) return { change: 'new', delta: {} };
  
  const delta = {};
  
  // Compare numeric metrics
  for (const key of Object.keys(current.summary || {})) {
    const curr = current.summary[key];
    const prev = previous.summary?.[key];
    
    if (typeof curr === 'number' && typeof prev === 'number') {
      const change = curr - prev;
      const pct = prev !== 0 ? ((change / prev) * 100).toFixed(1) : 0;
      delta[key] = { current: curr, previous: prev, change, percentChange: pct };
    }
  }
  
  return {
    change: current.passed === previous.passed ? 'stable' : (current.passed ? 'improved' : 'regressed'),
    delta
  };
}

module.exports = {
  saveResult,
  getHistory,
  getAllResults,
  compare,
  RESULTS_DIR
};

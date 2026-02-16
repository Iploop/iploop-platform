#!/usr/bin/env node
// ─── Browser Load Test (Puppeteer) ─────────────────────────────────────────────
// Persistent Chromium instance, reuses tabs, realistic browsing simulation.
// Usage: node browser-load-test.js [duration_min] [concurrency]
// ────────────────────────────────────────────────────────────────────────────────

const puppeteer = require('puppeteer');
const fs = require('fs');
const path = require('path');

const PROXY = process.env.PROXY || 'http://gateway.iploop.io:8880';
const DURATION_MIN = parseInt(process.argv[2]) || 10;
const CONCURRENCY = parseInt(process.argv[3]) || 5;
const DURATION_MS = DURATION_MIN * 60 * 1000;

const SITES = [
  'https://www.bbc.com',
  'https://www.cnn.com',
  'https://www.reuters.com',
  'https://www.nytimes.com',
  'https://www.theguardian.com',
  'https://news.ycombinator.com',
  'https://www.washingtonpost.com',
  'https://www.aljazeera.com',
  'https://www.bloomberg.com',
  'https://techcrunch.com',
  'https://www.wired.com',
  'https://arstechnica.com',
  'https://www.nbcnews.com',
  'https://www.espn.com',
  'https://www.wikipedia.org',
  'https://www.amazon.com',
  'https://www.reddit.com',
  'https://www.stackoverflow.com',
  'https://www.github.com',
  'https://httpbin.org/ip',
  'https://api.ipify.org',
  'https://ipinfo.io/ip',
];

// ─── Stats ──────────────────────────────────────────────────────────────────────

const stats = {
  total: 0,
  success: 0,
  fail: 0,
  timeout: 0,
  bytes: 0,
  latencies: [],
  exitIPs: {},
  errors: {},
  siteResults: {},
};

const startTime = Date.now();
const endTime = startTime + DURATION_MS;

const LOG_DIR = path.join(__dirname, 'results');
const TS = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
const CSV_FILE = path.join(LOG_DIR, `browser-test-${TS}.csv`);
const LOG_FILE = path.join(LOG_DIR, `browser-test-${TS}.log`);

fs.mkdirSync(LOG_DIR, { recursive: true });
fs.writeFileSync(CSV_FILE, 'timestamp,site,status,time_ms,size_bytes,exit_ip,result\n');

function log(msg) {
  const line = `[${new Date().toISOString().slice(11, 19)}] ${msg}`;
  console.log(line);
  fs.appendFileSync(LOG_FILE, line + '\n');
}

function pickRandom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function shortSite(url) {
  return url.replace(/^https?:\/\//, '').replace(/\/.*/, '');
}

// ─── Worker: Navigate a tab ─────────────────────────────────────────────────────

async function doRequest(page, workerId) {
  const site = pickRandom(SITES);
  const short = shortSite(site);
  const ts = new Date().toISOString();
  const t0 = Date.now();

  let status = 0;
  let sizeBytes = 0;
  let exitIP = '';
  let result = 'FAIL';
  let errMsg = '';

  try {
    const resp = await page.goto(site, {
      waitUntil: 'domcontentloaded',
      timeout: 30000,
    });

    const elapsed = Date.now() - t0;
    status = resp ? resp.status() : 0;

    // Get page size
    try {
      const content = await page.content();
      sizeBytes = Buffer.byteLength(content, 'utf8');
    } catch (_) {}

    // Try to extract exit IP from IP-check sites
    if (site.includes('ipify') || site.includes('ipinfo') || site.includes('httpbin') || site.includes('ifconfig')) {
      try {
        const bodyText = await page.evaluate(() => document.body?.innerText?.trim() || '');
        const ipMatch = bodyText.match(/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
        if (ipMatch) exitIP = ipMatch[1];
      } catch (_) {}
    }

    if (status >= 200 && status < 400) {
      result = 'OK';
      stats.success++;
      stats.bytes += sizeBytes;
      stats.latencies.push(elapsed);
    } else {
      result = 'FAIL';
      stats.fail++;
      errMsg = `HTTP ${status}`;
    }

    if (result !== 'OK') {
      log(`  ❌ W${workerId} ${result} ${short} → ${status} (${elapsed}ms)`);
    }

    // CSV
    fs.appendFileSync(CSV_FILE, `${ts},${short},${status},${elapsed},${sizeBytes},${exitIP},${result}\n`);

  } catch (err) {
    const elapsed = Date.now() - t0;
    errMsg = err.message || String(err);

    if (errMsg.includes('timeout') || errMsg.includes('Timeout')) {
      result = 'TIMEOUT';
      stats.timeout++;
    } else {
      result = 'FAIL';
      stats.fail++;
    }

    const shortErr = errMsg.slice(0, 80);
    stats.errors[shortErr] = (stats.errors[shortErr] || 0) + 1;
    log(`  ❌ W${workerId} ${result} ${short} (${elapsed}ms) ${shortErr}`);
    fs.appendFileSync(CSV_FILE, `${ts},${short},0,${elapsed},0,,${result}\n`);
  }

  stats.total++;

  // Track site-level stats
  if (!stats.siteResults[short]) stats.siteResults[short] = { ok: 0, fail: 0 };
  if (result === 'OK') stats.siteResults[short].ok++;
  else stats.siteResults[short].fail++;

  // Track exit IPs
  if (exitIP) {
    stats.exitIPs[exitIP] = (stats.exitIPs[exitIP] || 0) + 1;
  }
}

// ─── Worker loop ────────────────────────────────────────────────────────────────

async function workerLoop(browser, workerId) {
  const page = await browser.newPage();

  // Set realistic viewport + UA
  await page.setViewport({ width: 1366, height: 768 });
  await page.setUserAgent(
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
  );

  // Block heavy resources for speed
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const type = req.resourceType();
    if (['image', 'media', 'font', 'stylesheet'].includes(type)) {
      req.abort();
    } else {
      req.continue();
    }
  });

  while (Date.now() < endTime) {
    await doRequest(page, workerId);
    // Random delay 1-3s between requests (realistic browsing)
    await new Promise((r) => setTimeout(r, 1000 + Math.random() * 2000));
  }

  await page.close();
}

// ─── Stats printer ──────────────────────────────────────────────────────────────

function printStats() {
  const elapsed = ((Date.now() - startTime) / 1000).toFixed(0);
  const remaining = Math.max(0, ((endTime - Date.now()) / 1000)).toFixed(0);
  const rate = stats.total > 0 ? ((stats.success / stats.total) * 100).toFixed(1) : '0.0';
  const rps = elapsed > 0 ? (stats.total / elapsed).toFixed(2) : '0';
  const avgLatency =
    stats.latencies.length > 0
      ? (stats.latencies.reduce((a, b) => a + b, 0) / stats.latencies.length).toFixed(0)
      : 'N/A';
  const p50 = percentile(stats.latencies, 50);
  const p95 = percentile(stats.latencies, 95);
  const mb = (stats.bytes / 1048576).toFixed(1);
  const uniqueIPs = Object.keys(stats.exitIPs).length;

  log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
  log(`  ${elapsed}s elapsed | ${remaining}s remaining | ${CONCURRENCY} workers`);
  log(`  Total: ${stats.total} | ✅ ${stats.success} | ❌ ${stats.fail} | ⏱ ${stats.timeout} | Rate: ${rate}%`);
  log(`  Avg: ${avgLatency}ms | P50: ${p50}ms | P95: ${p95}ms | RPS: ${rps}`);
  log(`  Data: ${mb}MB | Unique IPs: ${uniqueIPs}`);
  log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
}

function percentile(arr, p) {
  if (arr.length === 0) return 'N/A';
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.floor((p / 100) * sorted.length);
  return sorted[Math.min(idx, sorted.length - 1)];
}

// ─── Main ───────────────────────────────────────────────────────────────────────

async function main() {
  log('╔══════════════════════════════════════════════════╗');
  log(`║  Browser Load Test — ${DURATION_MIN}min, ${CONCURRENCY} tabs        ║`);
  log(`║  Proxy: ${PROXY}              ║`);
  log(`║  Sites: ${SITES.length} targets                        ║`);
  log('╚══════════════════════════════════════════════════╝');
  log('');

  const browser = await puppeteer.launch({
    executablePath: '/snap/bin/chromium',
    headless: 'new',
    args: [
      `--proxy-server=${PROXY}`,
      '--no-sandbox',
      '--disable-gpu',
      '--disable-dev-shm-usage',
      '--disable-extensions',
      '--disable-background-networking',
      '--ignore-certificate-errors',
    ],
  });

  log(`Browser launched (PID ${browser.process().pid}), starting ${CONCURRENCY} workers...`);
  log('');

  // Stats printer every 30s
  const statsInterval = setInterval(printStats, 30000);

  // Launch workers
  const workers = [];
  for (let i = 0; i < CONCURRENCY; i++) {
    workers.push(workerLoop(browser, i));
  }

  await Promise.all(workers);
  clearInterval(statsInterval);

  // ─── Final Report ──────────────────────────────────────────────────────────

  log('');
  log('╔══════════════════════════════════════════════════╗');
  log('║              FINAL RESULTS                      ║');
  log('╚══════════════════════════════════════════════════╝');
  printStats();

  // Latency distribution
  if (stats.latencies.length > 0) {
    const buckets = { '<1s': 0, '1-2s': 0, '2-5s': 0, '5-10s': 0, '10-20s': 0, '20s+': 0 };
    for (const l of stats.latencies) {
      if (l < 1000) buckets['<1s']++;
      else if (l < 2000) buckets['1-2s']++;
      else if (l < 5000) buckets['2-5s']++;
      else if (l < 10000) buckets['5-10s']++;
      else if (l < 20000) buckets['10-20s']++;
      else buckets['20s+']++;
    }
    log('');
    log('Latency distribution:');
    for (const [b, c] of Object.entries(buckets)) {
      if (c > 0) log(`  ${b}: ${c}`);
    }
  }

  // Top exit IPs
  const ips = Object.entries(stats.exitIPs).sort((a, b) => b[1] - a[1]);
  if (ips.length > 0) {
    log('');
    log('Exit IPs:');
    for (const [ip, count] of ips.slice(0, 10)) {
      log(`  ${ip}: ${count} requests`);
    }
  }

  // Per-site breakdown
  const sites = Object.entries(stats.siteResults).sort((a, b) => (b[1].ok + b[1].fail) - (a[1].ok + a[1].fail));
  if (sites.length > 0) {
    log('');
    log('Per-site results:');
    for (const [site, r] of sites) {
      const total = r.ok + r.fail;
      const pct = total > 0 ? ((r.ok / total) * 100).toFixed(0) : '0';
      log(`  ${site}: ${r.ok}/${total} (${pct}%)`);
    }
  }

  // Top errors
  const errs = Object.entries(stats.errors).sort((a, b) => b[1] - a[1]);
  if (errs.length > 0) {
    log('');
    log('Top errors:');
    for (const [err, count] of errs.slice(0, 5)) {
      log(`  [${count}x] ${err}`);
    }
  }

  log('');
  log(`CSV: ${CSV_FILE}`);
  log(`Log: ${LOG_FILE}`);

  await browser.close();
}

main().catch((err) => {
  console.error('Fatal:', err);
  process.exit(1);
});

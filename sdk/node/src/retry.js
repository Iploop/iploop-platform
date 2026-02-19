const crypto = require('crypto');
const RETRYABLE_STATUS = new Set([403, 407, 429, 500, 502, 503, 504]);
function newSessionId() { return crypto.randomBytes(8).toString('hex'); }
function shouldRetry(err, statusCode) {
  if (statusCode && RETRYABLE_STATUS.has(statusCode)) return true;
  if (err && (err.code === 'ECONNRESET' || err.code === 'ECONNREFUSED' || err.code === 'ETIMEDOUT' || err.code === 'EPIPE')) return true;
  return false;
}
async function retryRequest(fn, retries = 3, delay = 1000) {
  let lastErr;
  for (let i = 0; i < retries; i++) {
    try {
      const result = await fn(i);
      if (result && result.status && shouldRetry(null, result.status)) {
        if (i < retries - 1) { await sleep(delay * (i + 1)); continue; }
      }
      return result;
    } catch (e) {
      lastErr = e;
      if (shouldRetry(e) && i < retries - 1) { await sleep(delay * (i + 1)); continue; }
      if (i === retries - 1) break;
      throw e;
    }
  }
  throw lastErr;
}
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }
module.exports = { retryRequest, newSessionId, shouldRetry };

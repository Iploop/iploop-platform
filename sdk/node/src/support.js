const https = require('https');
const { AuthError, QuotaExceeded } = require('./exceptions');

class SupportClient {
  constructor(apiKey, apiBase) {
    this.apiKey = apiKey;
    this.apiBase = apiBase.replace(/\/+$/, '');
    this.headers = { 'Authorization': `Bearer ${apiKey}`, 'Content-Type': 'application/json' };
  }

  _request(method, path, body) {
    return new Promise((resolve, reject) => {
      const url = new URL(this.apiBase + path);
      const opts = {
        hostname: url.hostname, port: url.port || 443, path: url.pathname,
        method, headers: { ...this.headers }, timeout: 15000,
        rejectUnauthorized: true,
      };
      if (body) {
        const data = JSON.stringify(body);
        opts.headers['Content-Length'] = Buffer.byteLength(data);
      }
      const req = https.request(opts, res => {
        let chunks = [];
        res.on('data', c => chunks.push(c));
        res.on('end', () => {
          const text = Buffer.concat(chunks).toString();
          if (res.statusCode === 401) return reject(new AuthError());
          if (res.statusCode >= 400) return reject(new Error(`HTTP ${res.statusCode}: ${text}`));
          try { resolve(JSON.parse(text)); } catch { resolve(text); }
        });
      });
      req.on('error', reject);
      req.on('timeout', () => { req.destroy(); reject(new Error('Request timeout')); });
      if (body) req.write(JSON.stringify(body));
      req.end();
    });
  }

  async usage() {
    const data = await this._request('GET', '/api/support/diagnose');
    this._checkQuota(data);
    return data;
  }
  status() { return this._request('GET', '/api/support/status'); }
  ask(question) { return this._request('POST', '/api/support/ask', { question }); }
  countries() { return this._request('GET', '/api/support/countries'); }

  _checkQuota(data) {
    try {
      const used = data.used_gb || 0;
      const total = used + (data.remaining_gb || 999);
      if (total > 0) {
        const pct = used / total * 100;
        if (pct >= 100) throw new QuotaExceeded();
        if (pct >= 80) process.stderr.write(`⚠️  IPLoop: ${pct.toFixed(0)}% bandwidth used. Upgrade at https://iploop.io/pricing\n`);
      }
    } catch (e) { if (e instanceof QuotaExceeded) throw e; }
  }
}
module.exports = { SupportClient };

const http = require('http');
const https = require('https');
const zlib = require('zlib');
const { getHeaders } = require('./headers');
const { retryRequest, newSessionId } = require('./retry');
const { SupportClient } = require('./support');
const { StickySession } = require('./session');
const { AuthError, ProxyError, TimeoutError } = require('./exceptions');

class IPLoop {
  constructor(apiKey, opts = {}) {
    if (!apiKey) throw new AuthError('API key is required');
    this.apiKey = apiKey;
    this.proxyHost = 'gateway.iploop.io';
    this.proxyPort = 8880;
    this.apiBase = 'https://gateway.iploop.io:9443';
    this._country = opts.country || null;
    this._city = opts.city || null;
    this._debug = opts.debug || false;
    this._support = new SupportClient(apiKey, this.apiBase);
  }

  _buildAuth(opts = {}) {
    const parts = [this.apiKey];
    const c = opts.country || this._country;
    if (c) parts.push(`country-${c.toLowerCase()}`);
    const ci = opts.city || this._city;
    if (ci) parts.push(`city-${ci.toLowerCase()}`);
    if (opts.session) parts.push(`session-${opts.session}`);
    return parts.join(':');
  }

  _log(...args) { if (this._debug) console.error('[IPLoop]', ...args); }

  fetch(url, opts = {}) {
    const retries = opts.retries || 3;
    const timeout = (opts.timeout || 30) * 1000;
    const country = opts.country || this._country || 'US';
    const hdrs = { ...getHeaders(country), ...(opts.headers || {}) };

    return retryRequest(async (attempt) => {
      const sid = opts._noRotate ? opts.session : (opts.session || newSessionId());
      const auth = this._buildAuth({ ...opts, session: sid });
      const start = Date.now();

      const result = await this._connectAndFetch(url, auth, hdrs, timeout);
      const elapsed = ((Date.now() - start) / 1000).toFixed(2);
      this._log(`GET ${url} → ${result.status} (${elapsed}s) country=${country} session=${sid}`);
      return result;
    }, retries);
  }

  _connectAndFetch(url, auth, headers, timeout) {
    return new Promise((resolve, reject) => {
      const parsed = new URL(url);
      const isHttps = parsed.protocol === 'https:';
      const targetHost = parsed.hostname;
      const targetPort = parsed.port || (isHttps ? 443 : 80);

      const connectReq = http.request({
        host: this.proxyHost,
        port: this.proxyPort,
        method: 'CONNECT',
        path: `${targetHost}:${targetPort}`,
        headers: {
          'Host': `${targetHost}:${targetPort}`,
          'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        },
        timeout,
      });

      connectReq.on('connect', (res, socket) => {
        if (res.statusCode !== 200) {
          socket.destroy();
          return reject(new ProxyError(`CONNECT failed: ${res.statusCode}`));
        }

        const reqOpts = {
          hostname: targetHost,
          port: targetPort,
          path: parsed.pathname + (parsed.search || ''),
          method: 'GET',
          headers: { ...headers, Host: targetHost },
          socket,
          agent: false,
          timeout,
        };

        const mod = isHttps ? https : http;
        const req = (isHttps ? mod.request({ ...reqOpts, servername: targetHost }) : mod.request(reqOpts));

        req.on('response', (response) => {
          const chunks = [];
          const encoding = response.headers['content-encoding'];
          let stream = response;
          if (encoding === 'gzip') stream = response.pipe(zlib.createGunzip());
          else if (encoding === 'deflate') stream = response.pipe(zlib.createInflate());
          else if (encoding === 'br') stream = response.pipe(zlib.createBrotliDecompress());

          stream.on('data', c => chunks.push(c));
          stream.on('end', () => {
            const body = Buffer.concat(chunks).toString();
            resolve({ status: response.statusCode, headers: response.headers, body });
          });
          stream.on('error', reject);
        });
        req.on('error', e => reject(new ProxyError(`Request failed: ${e.message}`)));
        req.on('timeout', () => { req.destroy(); reject(new TimeoutError()); });
        req.end();
      });

      connectReq.on('error', e => reject(new ProxyError(`CONNECT error: ${e.message}`)));
      connectReq.on('timeout', () => { connectReq.destroy(); reject(new TimeoutError()); });
      connectReq.end();
    });
  }

  session(opts = {}) {
    return new StickySession(this, opts.sessionId || newSessionId(), opts.country, opts.city);
  }

  usage() { return this._support.usage(); }
  status() { return this._support.status(); }
  ask(question) { return this._support.ask(question); }
  countries() { return this._support.countries(); }
}

module.exports = { IPLoop };

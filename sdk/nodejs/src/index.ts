/**
 * IPLoop Node.js SDK
 * Official SDK for IPLoop residential proxy service
 * https://iploop.io
 */

import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import { SocksProxyAgent } from 'socks-proxy-agent';
import { HttpsProxyAgent } from 'https-proxy-agent';
import { randomUUID } from 'crypto';

// ── Types ────────────────────────────────────────────────────

export interface IPLoopConfig {
  /** Your IPLoop API key */
  apiKey: string;
  /** Proxy gateway hostname (default: gateway.iploop.io) */
  proxyHost?: string;
  /** HTTP proxy port (default: 8880) */
  httpPort?: number;
  /** SOCKS5 proxy port (default: 1080) */
  socksPort?: number;
  /** Management API URL (default: https://gateway.iploop.io:9443) */
  apiUrl?: string;
  /** Default request timeout in ms (default: 30000) */
  timeout?: number;
  /** Default target country code */
  country?: string;
  /** Default target city */
  city?: string;
  /** Enable debug logging */
  debug?: boolean;
}

export interface ProxyOptions {
  /** Target country code (e.g. "US", "DE", "JP") */
  country?: string;
  /** Target city (e.g. "miami", "london") */
  city?: string;
  /** Session ID for sticky sessions (same IP across requests) */
  session?: string;
  /** Proxy protocol (default: "http") */
  protocol?: 'http' | 'socks5';
  /** Enable JavaScript rendering */
  render?: boolean;
}

export interface FetchResult {
  url: string;
  status: number;
  success: boolean;
  sizeKb: number;
  error?: string;
}

export interface UsageSummary {
  totalBytes: number;
  totalRequests: number;
  planLimitBytes: number;
  usagePercent: number;
  estimatedCost: number;
}

export interface DailyUsage {
  date: string;
  bytesTransferred: number;
  requestCount: number;
  successRate: number;
}

// ── Errors ───────────────────────────────────────────────────

export class IPLoopError extends Error {
  constructor(
    message: string,
    public statusCode?: number,
    public response?: any
  ) {
    super(message);
    this.name = 'IPLoopError';
  }
}

export class AuthenticationError extends IPLoopError {
  constructor(message: string = 'Invalid API key') {
    super(message, 401);
    this.name = 'AuthenticationError';
  }
}

export class RateLimitError extends IPLoopError {
  constructor(
    message: string = 'Rate limit exceeded',
    public retryAfter?: number
  ) {
    super(message, 429);
    this.name = 'RateLimitError';
  }
}

export class QuotaExceededError extends IPLoopError {
  constructor(message: string = 'Quota exceeded. Upgrade at https://iploop.io/pricing') {
    super(message, 402);
    this.name = 'QuotaExceededError';
  }
}

export class ProxyError extends IPLoopError {
  constructor(message: string = 'Proxy connection failed') {
    super(message);
    this.name = 'ProxyError';
  }
}

// ── Sticky Session ───────────────────────────────────────────

export class StickySession {
  private readonly client: IPLoopClient;
  readonly sessionId: string;
  readonly country?: string;
  readonly city?: string;

  constructor(client: IPLoopClient, sessionId: string, country?: string, city?: string) {
    this.client = client;
    this.sessionId = sessionId;
    this.country = country;
    this.city = city;
  }

  async get<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.get<T>(url, { country: this.country, city: this.city, session: this.sessionId }, config);
  }

  async post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.client.post<T>(url, data, { country: this.country, city: this.city, session: this.sessionId }, config);
  }
}

// ── Chrome Fingerprint ───────────────────────────────────────

const CHROME_VERSIONS = ['120.0.0.0', '121.0.0.0', '122.0.0.0', '123.0.0.0', '124.0.0.0', '125.0.0.0', '126.0.0.0'];

const LANG_MAP: Record<string, string> = {
  US: 'en-US,en;q=0.9', GB: 'en-GB,en;q=0.9', DE: 'de-DE,de;q=0.9,en;q=0.8',
  FR: 'fr-FR,fr;q=0.9,en;q=0.8', JP: 'ja-JP,ja;q=0.9,en;q=0.8', BR: 'pt-BR,pt;q=0.9,en;q=0.8',
  KR: 'ko-KR,ko;q=0.9,en;q=0.8', IN: 'en-IN,en;q=0.9,hi;q=0.8', ES: 'es-ES,es;q=0.9,en;q=0.8',
  IT: 'it-IT,it;q=0.9,en;q=0.8', NL: 'nl-NL,nl;q=0.9,en;q=0.8', AU: 'en-AU,en;q=0.9',
};

function chromeFingerprint(country = 'US'): Record<string, string> {
  const ver = CHROME_VERSIONS[Math.floor(Math.random() * CHROME_VERSIONS.length)];
  const major = ver.split('.')[0];
  const lang = LANG_MAP[country.toUpperCase()] || 'en-US,en;q=0.9';
  return {
    'User-Agent': `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${ver} Safari/537.36`,
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8',
    'Accept-Language': lang,
    'Accept-Encoding': 'gzip, deflate, br',
    'Connection': 'keep-alive',
    'Upgrade-Insecure-Requests': '1',
    'Sec-Fetch-Dest': 'document',
    'Sec-Fetch-Mode': 'navigate',
    'Sec-Fetch-Site': 'none',
    'Sec-Fetch-User': '?1',
    'Sec-Ch-Ua': `"Chromium";v="${major}", "Google Chrome";v="${major}", "Not-A.Brand";v="99"`,
    'Sec-Ch-Ua-Mobile': '?0',
    'Sec-Ch-Ua-Platform': '"Windows"',
    'Cache-Control': 'max-age=0',
  };
}

// ── Retryable statuses ───────────────────────────────────────

const RETRYABLE_STATUSES = new Set([403, 407, 429, 500, 502, 503, 504]);

function newSessionId(): string {
  return randomUUID().replace(/-/g, '').slice(0, 16);
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// ── Main Client ──────────────────────────────────────────────

export class IPLoopClient {
  private readonly apiKey: string;
  private readonly proxyHost: string;
  private readonly httpPort: number;
  private readonly socksPort: number;
  private readonly apiUrl: string;
  private readonly timeout: number;
  private readonly defaultCountry?: string;
  private readonly defaultCity?: string;
  private readonly debug: boolean;
  private readonly apiClient: AxiosInstance;
  private stats = { requests: 0, success: 0, errors: 0, totalTime: 0 };

  constructor(config: IPLoopConfig) {
    if (!config.apiKey) throw new AuthenticationError('API key is required');
    this.apiKey = config.apiKey;
    this.proxyHost = config.proxyHost || 'gateway.iploop.io';
    this.httpPort = config.httpPort || 8880;
    this.socksPort = config.socksPort || 1080;
    this.apiUrl = config.apiUrl || 'https://gateway.iploop.io:9443';
    this.timeout = config.timeout || 30000;
    this.defaultCountry = config.country;
    this.defaultCity = config.city;
    this.debug = config.debug || false;

    this.apiClient = axios.create({
      baseURL: this.apiUrl,
      timeout: this.timeout,
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
      },
    });
  }

  // ── Proxy URL & Auth ────────────────────────────────────

  private buildAuth(options: ProxyOptions = {}): string {
    const parts = [this.apiKey];
    const country = options.country || this.defaultCountry;
    const city = options.city || this.defaultCity;
    if (country) parts.push(`country-${country.toLowerCase()}`);
    if (city) parts.push(`city-${city.toLowerCase()}`);
    if (options.session) parts.push(`session-${options.session}`);
    if (options.render) parts.push('render-1');
    return parts.join('-');
  }

  /**
   * Get proxy URL for use with other HTTP libraries (e.g. puppeteer, playwright, got).
   */
  getProxyUrl(options: ProxyOptions = {}): string {
    const auth = this.buildAuth(options);
    if (options.protocol === 'socks5') {
      return `socks5://user:${auth}@${this.proxyHost}:${this.socksPort}`;
    }
    return `http://user:${auth}@${this.proxyHost}:${this.httpPort}`;
  }

  /**
   * Get an HTTP/SOCKS agent for use with axios or other libraries.
   */
  getProxyAgent(options: ProxyOptions = {}): HttpsProxyAgent<string> | SocksProxyAgent {
    const url = this.getProxyUrl(options);
    if (options.protocol === 'socks5') return new SocksProxyAgent(url);
    return new HttpsProxyAgent(url);
  }

  // ── Request Methods ─────────────────────────────────────

  /**
   * Fetch a URL through the residential proxy with auto-retry and smart headers.
   */
  async fetch<T = any>(
    url: string,
    options: ProxyOptions = {},
    config: AxiosRequestConfig = {},
    retries = 3,
  ): Promise<AxiosResponse<T>> {
    const country = options.country || this.defaultCountry || 'US';
    const headers = { ...chromeFingerprint(country), ...config.headers };
    const agent = this.getProxyAgent(options);

    this.stats.requests++;
    const start = Date.now();
    let lastError: Error | undefined;

    for (let attempt = 0; attempt < retries; attempt++) {
      const sid = options.session || newSessionId();
      const proxyOpts = { ...options, session: sid };
      const proxyAgent = options.session ? agent : this.getProxyAgent(proxyOpts);

      try {
        const resp = await axios({
          method: (config.method as any) || 'GET',
          url,
          ...config,
          headers,
          httpsAgent: proxyAgent,
          httpAgent: proxyAgent,
          timeout: this.timeout,
        });

        const elapsed = Date.now() - start;
        if (this.debug) {
          console.log(`IPLoop: ${config.method || 'GET'} ${url} → ${resp.status} (${elapsed}ms) country=${country} session=${sid}`);
        }

        if (RETRYABLE_STATUSES.has(resp.status) && attempt < retries - 1) {
          await sleep(1000 * (attempt + 1));
          continue;
        }

        this.stats.success++;
        this.stats.totalTime += elapsed;
        return resp;
      } catch (err: any) {
        lastError = err;
        if (attempt < retries - 1) {
          await sleep(1000 * (attempt + 1));
        }
      }
    }

    this.stats.errors++;
    this.stats.totalTime += Date.now() - start;

    if (lastError?.message?.includes('timeout')) {
      throw new IPLoopError(`All ${retries} retries timed out for ${url}`);
    }
    throw new ProxyError(`Proxy connection failed after ${retries} retries: ${lastError?.message}`);
  }

  /** GET request through proxy. */
  async get<T = any>(url: string, options: ProxyOptions = {}, config: AxiosRequestConfig = {}): Promise<AxiosResponse<T>> {
    return this.fetch<T>(url, options, { ...config, method: 'GET' });
  }

  /** POST request through proxy. */
  async post<T = any>(url: string, data?: any, options: ProxyOptions = {}, config: AxiosRequestConfig = {}): Promise<AxiosResponse<T>> {
    return this.fetch<T>(url, options, { ...config, method: 'POST', data });
  }

  /** PUT request through proxy. */
  async put<T = any>(url: string, data?: any, options: ProxyOptions = {}, config: AxiosRequestConfig = {}): Promise<AxiosResponse<T>> {
    return this.fetch<T>(url, options, { ...config, method: 'PUT', data });
  }

  /** DELETE request through proxy. */
  async delete<T = any>(url: string, options: ProxyOptions = {}, config: AxiosRequestConfig = {}): Promise<AxiosResponse<T>> {
    return this.fetch<T>(url, options, { ...config, method: 'DELETE' });
  }

  // ── Sticky Sessions ─────────────────────────────────────

  /**
   * Create a sticky session — all requests reuse the same proxy IP.
   */
  session(sessionId?: string, country?: string, city?: string): StickySession {
    return new StickySession(this, sessionId || newSessionId(), country || this.defaultCountry, city || this.defaultCity);
  }

  // ── Batch / Concurrent ─────────────────────────────────

  /**
   * Fetch multiple URLs concurrently through the proxy.
   */
  async fetchAll(urls: string[], options: ProxyOptions = {}, concurrency = 10): Promise<FetchResult[]> {
    const results: FetchResult[] = [];
    const queue = [...urls];

    const worker = async () => {
      while (queue.length > 0) {
        const url = queue.shift()!;
        try {
          const resp = await this.get(url, options);
          results.push({ url, status: resp.status, success: true, sizeKb: Math.round(JSON.stringify(resp.data).length / 1024) });
        } catch (err: any) {
          results.push({ url, status: 0, success: false, sizeKb: 0, error: err.message });
        }
      }
    };

    await Promise.all(Array.from({ length: Math.min(concurrency, urls.length) }, () => worker()));
    return results;
  }

  // ── Stats ───────────────────────────────────────────────

  /**
   * Get request statistics.
   */
  getStats() {
    const avg = this.stats.requests > 0 ? this.stats.totalTime / this.stats.requests : 0;
    const rate = this.stats.requests > 0 ? (this.stats.success / this.stats.requests) * 100 : 0;
    return { ...this.stats, avgTime: Math.round(avg), successRate: Math.round(rate * 10) / 10 };
  }

  // ── Chrome Fingerprint ──────────────────────────────────

  /**
   * Get Chrome desktop fingerprint headers for a country.
   */
  fingerprint(country = 'US'): Record<string, string> {
    return chromeFingerprint(country);
  }

  // ── API Methods ─────────────────────────────────────────

  /** Check bandwidth usage and quota. */
  async getUsage(): Promise<UsageSummary> {
    const resp = await this.apiClient.get<UsageSummary>('/api/support/diagnose');
    return resp.data;
  }

  /** Check service status. */
  async getStatus(): Promise<any> {
    const resp = await this.apiClient.get('/api/support/status');
    return resp.data;
  }

  /** List available proxy countries. */
  async getCountries(): Promise<any> {
    const resp = await this.apiClient.get('/api/support/countries');
    return resp.data;
  }
}

// ── Version Check ────────────────────────────────────────────

const SDK_VERSION = '1.0.1';

function checkVersion(): void {
  try {
    const https = require('https');
    https.get('https://registry.npmjs.org/iploop/latest', { timeout: 3000 }, (res: any) => {
      let data = '';
      res.on('data', (chunk: string) => { data += chunk; });
      res.on('end', () => {
        try {
          const latest = JSON.parse(data).version;
          if (latest && latest !== SDK_VERSION) {
            console.warn(`\n⚠️  IPLoop v${latest} available (you have ${SDK_VERSION}). Run: npm update iploop\n`);
          }
        } catch {}
      });
    }).on('error', () => {});
  } catch {}
}

checkVersion();

export { SDK_VERSION };
export default IPLoopClient;

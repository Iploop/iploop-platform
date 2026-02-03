/**
 * IPLoop Node.js SDK
 * Official SDK for IPLoop residential proxy service
 */

import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import { SocksProxyAgent } from 'socks-proxy-agent';
import { HttpsProxyAgent } from 'https-proxy-agent';

export interface IPLoopConfig {
  apiKey: string;
  proxyHost?: string;
  httpPort?: number;
  socksPort?: number;
  apiUrl?: string;
  timeout?: number;
}

export interface ProxyOptions {
  country?: string;
  city?: string;
  session?: string;
  protocol?: 'http' | 'socks5';
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

export interface ApiKey {
  id: string;
  name: string;
  key: string;
  createdAt: string;
  lastUsed?: string;
}

export class IPLoopClient {
  private readonly apiKey: string;
  private readonly proxyHost: string;
  private readonly httpPort: number;
  private readonly socksPort: number;
  private readonly apiUrl: string;
  private readonly timeout: number;
  private readonly apiClient: AxiosInstance;

  static readonly DEFAULT_PROXY_HOST = 'proxy.iploop.io';
  static readonly DEFAULT_HTTP_PORT = 7777;
  static readonly DEFAULT_SOCKS_PORT = 1080;
  static readonly DEFAULT_API_URL = 'https://api.iploop.io';

  constructor(config: IPLoopConfig) {
    this.apiKey = config.apiKey;
    this.proxyHost = config.proxyHost || IPLoopClient.DEFAULT_PROXY_HOST;
    this.httpPort = config.httpPort || IPLoopClient.DEFAULT_HTTP_PORT;
    this.socksPort = config.socksPort || IPLoopClient.DEFAULT_SOCKS_PORT;
    this.apiUrl = config.apiUrl || IPLoopClient.DEFAULT_API_URL;
    this.timeout = config.timeout || 30000;

    this.apiClient = axios.create({
      baseURL: this.apiUrl,
      timeout: this.timeout,
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
      },
    });
  }

  /**
   * Get proxy URL for use with other libraries
   */
  getProxyUrl(options: ProxyOptions = {}): string {
    const username = this.buildUsername(options);
    const port = options.protocol === 'socks5' ? this.socksPort : this.httpPort;
    const protocol = options.protocol === 'socks5' ? 'socks5' : 'http';
    
    return `${protocol}://${username}:${this.apiKey}@${this.proxyHost}:${port}`;
  }

  /**
   * Get axios proxy agent for use with axios
   */
  getProxyAgent(options: ProxyOptions = {}): HttpsProxyAgent | SocksProxyAgent {
    const proxyUrl = this.getProxyUrl(options);
    
    if (options.protocol === 'socks5') {
      return new SocksProxyAgent(proxyUrl);
    }
    
    return new HttpsProxyAgent(proxyUrl);
  }

  /**
   * Make a GET request through the proxy
   */
  async get<T = any>(
    url: string,
    options: ProxyOptions = {},
    config: AxiosRequestConfig = {}
  ): Promise<AxiosResponse<T>> {
    const agent = this.getProxyAgent(options);
    return axios.get<T>(url, {
      ...config,
      timeout: this.timeout,
      httpsAgent: agent,
      httpAgent: agent,
    });
  }

  /**
   * Make a POST request through the proxy
   */
  async post<T = any>(
    url: string,
    data?: any,
    options: ProxyOptions = {},
    config: AxiosRequestConfig = {}
  ): Promise<AxiosResponse<T>> {
    const agent = this.getProxyAgent(options);
    return axios.post<T>(url, data, {
      ...config,
      timeout: this.timeout,
      httpsAgent: agent,
      httpAgent: agent,
    });
  }

  /**
   * Make a PUT request through the proxy
   */
  async put<T = any>(
    url: string,
    data?: any,
    options: ProxyOptions = {},
    config: AxiosRequestConfig = {}
  ): Promise<AxiosResponse<T>> {
    const agent = this.getProxyAgent(options);
    return axios.put<T>(url, data, {
      ...config,
      timeout: this.timeout,
      httpsAgent: agent,
      httpAgent: agent,
    });
  }

  /**
   * Make a DELETE request through the proxy
   */
  async delete<T = any>(
    url: string,
    options: ProxyOptions = {},
    config: AxiosRequestConfig = {}
  ): Promise<AxiosResponse<T>> {
    const agent = this.getProxyAgent(options);
    return axios.delete<T>(url, {
      ...config,
      timeout: this.timeout,
      httpsAgent: agent,
      httpAgent: agent,
    });
  }

  // API Methods

  /**
   * Get current usage statistics
   */
  async getUsage(): Promise<UsageSummary> {
    const response = await this.apiClient.get<UsageSummary>('/usage/summary');
    return response.data;
  }

  /**
   * Get daily usage breakdown
   */
  async getDailyUsage(days: number = 30): Promise<DailyUsage[]> {
    const response = await this.apiClient.get<DailyUsage[]>(`/usage/daily?days=${days}`);
    return response.data;
  }

  /**
   * List all API keys
   */
  async listApiKeys(): Promise<ApiKey[]> {
    const response = await this.apiClient.get<{ keys: ApiKey[] }>('/keys');
    return response.data.keys;
  }

  /**
   * Create a new API key
   */
  async createApiKey(name: string): Promise<ApiKey> {
    const response = await this.apiClient.post<ApiKey>('/keys', { name });
    return response.data;
  }

  /**
   * Delete an API key
   */
  async deleteApiKey(keyId: string): Promise<void> {
    await this.apiClient.delete(`/keys/${keyId}`);
  }

  /**
   * Get current subscription details
   */
  async getSubscription(): Promise<any> {
    const response = await this.apiClient.get('/subscription');
    return response.data;
  }

  private buildUsername(options: ProxyOptions): string {
    const parts = ['user'];
    
    if (options.country) {
      parts.push(`country-${options.country.toUpperCase()}`);
    }
    if (options.city) {
      parts.push(`city-${options.city.toLowerCase()}`);
    }
    if (options.session) {
      parts.push(`session-${options.session}`);
    }
    
    return parts.join('-');
  }
}

// Export default
export default IPLoopClient;

// Export errors
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
  constructor(
    message: string = 'Quota exceeded',
    public quotaType?: 'bandwidth' | 'requests'
  ) {
    super(message, 402);
    this.name = 'QuotaExceededError';
  }
}

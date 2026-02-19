/**
 * IPLoop Node.js SDK
 * Official SDK for IPLoop residential proxy service
 * https://iploop.io
 */
import { AxiosRequestConfig, AxiosResponse } from 'axios';
import { SocksProxyAgent } from 'socks-proxy-agent';
import { HttpsProxyAgent } from 'https-proxy-agent';
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
export declare class IPLoopError extends Error {
    statusCode?: number | undefined;
    response?: any | undefined;
    constructor(message: string, statusCode?: number | undefined, response?: any | undefined);
}
export declare class AuthenticationError extends IPLoopError {
    constructor(message?: string);
}
export declare class RateLimitError extends IPLoopError {
    retryAfter?: number | undefined;
    constructor(message?: string, retryAfter?: number | undefined);
}
export declare class QuotaExceededError extends IPLoopError {
    constructor(message?: string);
}
export declare class ProxyError extends IPLoopError {
    constructor(message?: string);
}
export declare class StickySession {
    private readonly client;
    readonly sessionId: string;
    readonly country?: string;
    readonly city?: string;
    constructor(client: IPLoopClient, sessionId: string, country?: string, city?: string);
    get<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
}
export declare class IPLoopClient {
    private readonly apiKey;
    private readonly proxyHost;
    private readonly httpPort;
    private readonly socksPort;
    private readonly apiUrl;
    private readonly timeout;
    private readonly defaultCountry?;
    private readonly defaultCity?;
    private readonly debug;
    private readonly apiClient;
    private stats;
    constructor(config: IPLoopConfig);
    private buildAuth;
    /**
     * Get proxy URL for use with other HTTP libraries (e.g. puppeteer, playwright, got).
     */
    getProxyUrl(options?: ProxyOptions): string;
    /**
     * Get an HTTP/SOCKS agent for use with axios or other libraries.
     */
    getProxyAgent(options?: ProxyOptions): HttpsProxyAgent<string> | SocksProxyAgent;
    /**
     * Fetch a URL through the residential proxy with auto-retry and smart headers.
     */
    fetch<T = any>(url: string, options?: ProxyOptions, config?: AxiosRequestConfig, retries?: number): Promise<AxiosResponse<T>>;
    /** GET request through proxy. */
    get<T = any>(url: string, options?: ProxyOptions, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    /** POST request through proxy. */
    post<T = any>(url: string, data?: any, options?: ProxyOptions, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    /** PUT request through proxy. */
    put<T = any>(url: string, data?: any, options?: ProxyOptions, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    /** DELETE request through proxy. */
    delete<T = any>(url: string, options?: ProxyOptions, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    /**
     * Create a sticky session — all requests reuse the same proxy IP.
     */
    session(sessionId?: string, country?: string, city?: string): StickySession;
    /**
     * Fetch multiple URLs concurrently through the proxy.
     */
    fetchAll(urls: string[], options?: ProxyOptions, concurrency?: number): Promise<FetchResult[]>;
    /**
     * Get request statistics.
     */
    getStats(): {
        avgTime: number;
        successRate: number;
        requests: number;
        success: number;
        errors: number;
        totalTime: number;
    };
    /**
     * Get Chrome desktop fingerprint headers for a country.
     */
    fingerprint(country?: string): Record<string, string>;
    /** Check bandwidth usage and quota. */
    getUsage(): Promise<UsageSummary>;
    /** Check service status. */
    getStatus(): Promise<any>;
    /** List available proxy countries. */
    getCountries(): Promise<any>;
}
export default IPLoopClient;
//# sourceMappingURL=index.d.ts.map
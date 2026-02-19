# IPLoop Node.js SDK

Official Node.js SDK for [IPLoop](https://iploop.io) — residential proxy service with millions of real IPs worldwide.

[![npm version](https://img.shields.io/npm/v/iploop.svg)](https://www.npmjs.com/package/iploop)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

## Installation

```bash
npm install iploop
```

## Quick Start

```typescript
import { IPLoopClient } from 'iploop';

const client = new IPLoopClient({ apiKey: 'your-api-key' });
const response = await client.get('https://httpbin.org/ip');
console.log(response.data);
```

## Features

- **Residential proxies** — millions of real IPs across 195+ countries
- **Geographic targeting** — country and city-level precision
- **Sticky sessions** — keep the same IP across multiple requests
- **Auto-retry** — failed requests automatically retry with fresh IPs
- **Smart headers** — Chrome fingerprint headers matched to target country
- **HTTP & SOCKS5** — both protocols supported
- **Concurrent fetching** — batch requests with configurable concurrency
- **TypeScript** — full type definitions included

## Authentication

Get your API key from the [IPLoop Dashboard](https://iploop.io/dashboard).

```typescript
const client = new IPLoopClient({
  apiKey: 'your-api-key',
  country: 'US',       // default country for all requests
  debug: true,         // enable request logging
});
```

## Geographic Targeting

```typescript
// Target a specific country
const resp = await client.get('https://example.com', { country: 'DE' });

// Target a specific city
const resp = await client.get('https://example.com', { country: 'US', city: 'miami' });
```

## Sticky Sessions

Keep the same proxy IP across multiple requests:

```typescript
const session = client.session(undefined, 'US', 'newyork');

const page1 = await session.get('https://site.com/page1'); // same IP
const page2 = await session.get('https://site.com/page2'); // same IP
```

## HTTP Methods

```typescript
// GET
const resp = await client.get('https://httpbin.org/get');

// POST
const resp = await client.post('https://httpbin.org/post', { key: 'value' });

// PUT
const resp = await client.put('https://httpbin.org/put', { key: 'value' });

// DELETE
const resp = await client.delete('https://httpbin.org/delete');
```

## Concurrent Fetching

Fetch multiple URLs in parallel:

```typescript
const results = await client.fetchAll([
  'https://example.com/page1',
  'https://example.com/page2',
  'https://example.com/page3',
], { country: 'US' }, 10); // 10 concurrent workers

console.log(results);
// [{ url: '...', status: 200, success: true, sizeKb: 42 }, ...]
```

## SOCKS5 Support

```typescript
// Use SOCKS5 protocol
const resp = await client.get('https://example.com', { protocol: 'socks5' });

// Get SOCKS5 proxy URL for use with other libraries
const proxyUrl = client.getProxyUrl({ protocol: 'socks5', country: 'US' });
```

## Use with Other Libraries

Get the proxy URL or agent for use with Puppeteer, Playwright, Got, etc:

```typescript
// Get proxy URL string
const proxyUrl = client.getProxyUrl({ country: 'US', session: 'my-session' });
// → http://your-api-key:country-us:session-my-session@gateway.iploop.io:8880

// Get axios-compatible agent
const agent = client.getProxyAgent({ country: 'DE' });

// Use with Puppeteer
const browser = await puppeteer.launch({
  args: [`--proxy-server=${client.getProxyUrl({ country: 'US' })}`],
});
```

## Chrome Fingerprinting

Every request automatically includes 14 Chrome desktop headers matched to the target country. You can also get them directly:

```typescript
const headers = client.fingerprint('JP');
// Full Chrome headers with Japanese locale
```

## Request Statistics

```typescript
// After making requests...
console.log(client.getStats());
// { requests: 10, success: 9, errors: 1, totalTime: 23500, avgTime: 2350, successRate: 90.0 }
```

## API Methods

```typescript
// Check bandwidth usage
const usage = await client.getUsage();

// Service status
const status = await client.getStatus();

// Available countries
const countries = await client.getCountries();
```

## Error Handling

```typescript
import { AuthenticationError, QuotaExceededError, ProxyError, RateLimitError } from 'iploop';

try {
  const resp = await client.get('https://example.com');
} catch (err) {
  if (err instanceof AuthenticationError) {
    console.log('Invalid API key');
  } else if (err instanceof QuotaExceededError) {
    console.log('Upgrade at https://iploop.io/pricing');
  } else if (err instanceof ProxyError) {
    console.log('Proxy connection failed');
  } else if (err instanceof RateLimitError) {
    console.log('Rate limited, retry after:', err.retryAfter);
  }
}
```

## Debug Mode

```typescript
const client = new IPLoopClient({ apiKey: 'your-key', debug: true });
// Logs: IPLoop: GET https://example.com → 200 (450ms) country=US session=abc123
```

## Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `apiKey` | *required* | Your IPLoop API key |
| `proxyHost` | `gateway.iploop.io` | Proxy gateway hostname |
| `httpPort` | `8880` | HTTP proxy port |
| `socksPort` | `1080` | SOCKS5 proxy port |
| `timeout` | `30000` | Request timeout in ms |
| `country` | — | Default target country |
| `city` | — | Default target city |
| `debug` | `false` | Enable debug logging |

## Links

- **Website**: [iploop.io](https://iploop.io)
- **Dashboard**: [iploop.io/dashboard](https://iploop.io/dashboard)
- **Documentation**: [docs.iploop.io](https://docs.iploop.io)

## License

MIT

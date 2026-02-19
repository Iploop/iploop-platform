# IPLoop Node.js SDK

Residential proxy SDK — one-liner web fetching through millions of real IPs.

## Install

```bash
npm install iploop
```

## Quick Start

```javascript
const { IPLoop } = require('iploop');
const ip = new IPLoop('YOUR_API_KEY');

// One-liner fetch through US residential IP
const result = await ip.fetch('https://example.com', { country: 'US' });
console.log(result.status, result.body);

// Sticky session (same IP across requests)
const session = ip.session({ country: 'US', city: 'newyork' });
const page1 = await session.fetch('https://site.com/page1');
const page2 = await session.fetch('https://site.com/page2');

// Check usage
const usage = await ip.usage();

// Ask support
const answer = await ip.ask('how to handle captcha?');
```

## Features

- **Zero dependencies** — uses Node.js built-in `http`/`https` modules
- **Auto-retry** with IP rotation (3 attempts by default)
- **Smart headers** per country (28 countries, 12 Chrome UA versions)
- **Sticky sessions** for multi-page scraping
- **Support API** — usage, status, ask
- **Debug mode** — `new IPLoop('KEY', { debug: true })`

## API

### `new IPLoop(apiKey, options?)`
- `apiKey` — your API key from https://iploop.io
- `options.country` — default country code
- `options.city` — default city
- `options.debug` — enable debug logging

### `ip.fetch(url, options?)`
Returns `{ status, headers, body }`
- `country`, `city` — geo targeting
- `session` — session ID for sticky IP
- `headers` — custom headers (merged with smart defaults)
- `retries` — retry count (default: 3)
- `timeout` — seconds (default: 30)

### `ip.session(options?)`
Returns a `StickySession` with `.fetch()` that reuses the same IP.

### `ip.usage()` / `ip.status()` / `ip.ask(question)`
Support API endpoints.

## License

MIT — https://iploop.io

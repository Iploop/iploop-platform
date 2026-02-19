"""Async IPLoop client using aiohttp."""

import time
import logging
import asyncio

from .headers import get_headers
from .retry import new_session_id, RETRYABLE_STATUS
from .support import SupportClient
from .exceptions import AuthError, ProxyError, TimeoutError

logger = logging.getLogger("iploop")


class AsyncIPLoop:
    """Async residential proxy SDK."""

    def __init__(self, api_key, country=None, city=None, debug=False):
        if not api_key:
            raise AuthError("API key is required")
        self.api_key = api_key
        self.base_proxy = "proxy.iploop.io:8880"
        self.api_base = "https://gateway.iploop.io:9443"
        self._country = country
        self._city = city
        self._session = None
        self._support = SupportClient(api_key, self.api_base)

        if debug:
            logging.basicConfig(level=logging.DEBUG)
            logger.setLevel(logging.DEBUG)

    async def __aenter__(self):
        import aiohttp
        self._session = aiohttp.ClientSession()
        return self

    async def __aexit__(self, *args):
        if self._session:
            await self._session.close()
            self._session = None

    def _proxy_url(self, country=None, city=None, session=None, render=False):
        parts = [self.api_key]
        c = country or self._country
        if c:
            parts.append(f"country-{c.lower()}")
        ci = city or self._city
        if ci:
            parts.append(f"city-{ci.lower()}")
        if session:
            parts.append(f"session-{session}")
        if render:
            parts.append("render-1")
        auth = ":".join(parts)
        return f"http://{auth}@{self.base_proxy}"

    async def fetch(self, url, country=None, city=None, session=None, render=False,
                    headers=None, method="GET", data=None, timeout=30, retries=3):
        """Async fetch through residential proxy."""
        import aiohttp

        if not self._session:
            self._session = aiohttp.ClientSession()

        c = country or self._country or "US"
        h = get_headers(c)
        if headers:
            h.update(headers)

        last_exc = None
        for attempt in range(retries):
            sid = session or new_session_id()
            proxy = self._proxy_url(country=country, city=city, session=sid, render=render)
            start = time.time()
            try:
                async with self._session.request(
                    method, url, headers=h, proxy=proxy,
                    data=data, timeout=aiohttp.ClientTimeout(total=timeout)
                ) as resp:
                    elapsed = time.time() - start
                    logger.debug("%-3s %s → %d (%.2fs) country=%s",
                                 method, url, resp.status, elapsed, c)
                    if resp.status in RETRYABLE_STATUS and attempt < retries - 1:
                        await asyncio.sleep(1.0 * (attempt + 1))
                        continue
                    body = await resp.read()

                    class AsyncResponse:
                        def __init__(self, status, hdrs, content):
                            self.status_code = status
                            self.headers = hdrs
                            self.content = content
                            self.text = content.decode("utf-8", errors="replace")

                        def json(self):
                            import json
                            return json.loads(self.content)

                    return AsyncResponse(resp.status, dict(resp.headers), body)
            except (aiohttp.ClientError, asyncio.TimeoutError) as e:
                last_exc = e
                if attempt < retries - 1:
                    await asyncio.sleep(1.0 * (attempt + 1))

        if isinstance(last_exc, asyncio.TimeoutError):
            raise TimeoutError(f"All {retries} retries timed out for {url}")
        raise ProxyError(f"Proxy connection failed after {retries} retries: {last_exc}")

    async def get(self, url, **kwargs):
        return await self.fetch(url, method="GET", **kwargs)

    async def post(self, url, data=None, json=None, **kwargs):
        import json as json_mod
        if json is not None:
            kwargs.setdefault("headers", {})["Content-Type"] = "application/json"
            data = json_mod.dumps(json)
        return await self.fetch(url, method="POST", data=data, **kwargs)

    def usage(self):
        return self._support.usage()

    def status(self):
        return self._support.status()

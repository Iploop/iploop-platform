"""Main IPLoop client."""

import time
import logging
import requests

from .fingerprint import chrome_fingerprint
from .retry import retry_request, new_session_id
from .support import SupportClient
from .exceptions import AuthError, ProxyError, TimeoutError

logger = logging.getLogger("iploop")


class StickySession:
    """A session that reuses the same proxy IP."""

    def __init__(self, client, session_id, country=None, city=None):
        self._client = client
        self.session_id = session_id
        self.country = country or client._country
        self.city = city or client._city

    def fetch(self, url, **kwargs):
        kwargs.setdefault("country", self.country)
        kwargs.setdefault("city", self.city)
        kwargs["session"] = self.session_id
        kwargs["_no_rotate"] = True
        return self._client.fetch(url, **kwargs)

    def get(self, url, **kwargs):
        return self.fetch(url, method="GET", **kwargs)

    def post(self, url, data=None, json=None, **kwargs):
        kwargs["method"] = "POST"
        kwargs["data"] = data or json
        return self.fetch(url, **kwargs)


class IPLoop:
    """Residential proxy SDK — one-liner web fetching through millions of real IPs."""

    def __init__(self, api_key, country=None, city=None, debug=False):
        if not api_key:
            raise AuthError("API key is required")
        self.api_key = api_key
        self.base_proxy = "gateway.iploop.io:8880"
        self.api_base = "https://gateway.iploop.io:9443"
        self._country = country
        self._city = city
        self._support = SupportClient(api_key, self.api_base)
        self._stats = {"requests": 0, "success": 0, "errors": 0, "total_time": 0}

        if debug:
            logging.basicConfig(level=logging.DEBUG)
            logger.setLevel(logging.DEBUG)

    @property
    def stats(self):
        """Get request statistics."""
        avg = self._stats["total_time"] / max(self._stats["requests"], 1)
        return {
            **self._stats,
            "avg_time": round(avg, 2),
            "success_rate": round(self._stats["success"] / max(self._stats["requests"], 1) * 100, 1)
        }

    def _build_proxy_auth(self, country=None, city=None, session=None, render=False):
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
        return "-".join(parts)

    def _proxy_url(self, **kwargs):
        auth = self._build_proxy_auth(**kwargs)
        return {
            "http": f"http://user:{auth}@{self.base_proxy}",
            "https": f"http://user:{auth}@{self.base_proxy}",
        }

    def fetch(self, url, country=None, city=None, session=None, render=False,
              headers=None, method="GET", data=None, timeout=30, retries=3,
              _no_rotate=False):
        """Fetch a URL through residential proxy with auto-retry and smart headers."""
        c = country or self._country or "US"
        # Phase 9: auto-apply 14-header Chrome fingerprint
        h = chrome_fingerprint(c)
        if headers:
            h.update(headers)

        def do_request(attempt):
            sid = session if _no_rotate else (session or new_session_id())
            proxies = self._proxy_url(country=country, city=city, session=sid, render=render)
            req_start = time.time()
            resp = requests.request(method, url, headers=h, proxies=proxies,
                                    data=data, timeout=timeout, verify=True)
            elapsed = time.time() - req_start
            logger.debug("%-3s %s → %d (%.2fs) country=%s session=%s",
                         method, url, resp.status_code, elapsed, c, sid)
            return resp

        self._stats["requests"] += 1
        start = time.time()
        try:
            resp = retry_request(do_request, retries=retries)
            self._stats["success"] += 1
            self._stats["total_time"] += time.time() - start
            return resp
        except requests.exceptions.Timeout:
            self._stats["errors"] += 1
            raise TimeoutError(f"All {retries} retries timed out for {url}")
        except (requests.exceptions.ConnectionError, requests.exceptions.ProxyError) as e:
            self._stats["errors"] += 1
            raise ProxyError(f"Proxy connection failed after {retries} retries: {e}")
        except Exception:
            self._stats["errors"] += 1
            raise

    def get(self, url, **kwargs):
        """GET request through proxy."""
        return self.fetch(url, method="GET", **kwargs)

    def post(self, url, data=None, json=None, **kwargs):
        """POST request through proxy."""
        import json as json_mod
        if json is not None:
            kwargs.setdefault("headers", {})["Content-Type"] = "application/json"
            data = json_mod.dumps(json)
        return self.fetch(url, method="POST", data=data, **kwargs)

    def session(self, session_id=None, country=None, city=None):
        """Create a sticky session that reuses the same proxy IP."""
        return StickySession(self, session_id or new_session_id(), country, city)

    def batch(self, max_workers=10):
        """Create a batch fetcher for concurrent requests. Safe up to 25 workers."""
        from .concurrent import BatchFetcher
        return BatchFetcher(self, max_workers)

    def fingerprint(self, country="US"):
        """Get Chrome desktop fingerprint headers for a country."""
        return chrome_fingerprint(country)

    def usage(self):
        """Check bandwidth usage and quota."""
        return self._support.usage()

    def status(self):
        """Check service status."""
        return self._support.status()

    def ask(self, question):
        """Ask the support API a question."""
        return self._support.ask(question)

    def countries(self):
        """List available proxy countries."""
        return self._support.countries()

    # ── Site-specific presets ──────────────────────────────────

    @property
    def twitter(self):
        from .sites.twitter import Twitter
        return Twitter(self)

    @property
    def google(self):
        from .sites.google import Google
        return Google(self)

    @property
    def amazon(self):
        from .sites.amazon import Amazon
        return Amazon(self)

    @property
    def instagram(self):
        from .sites.instagram import Instagram
        return Instagram(self)

    @property
    def tiktok(self):
        from .sites.tiktok import TikTok
        return TikTok(self)

    @property
    def youtube(self):
        from .sites.youtube import YouTube
        return YouTube(self)

    @property
    def reddit(self):
        from .sites.reddit import Reddit
        return Reddit(self)

    @property
    def ebay(self):
        from .sites.ebay import eBay
        return eBay(self)

    @property
    def nasdaq(self):
        from .sites.nasdaq import Nasdaq
        return Nasdaq(self)

    @property
    def linkedin(self):
        from .sites.linkedin import LinkedIn
        return LinkedIn(self)

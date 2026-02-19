"""Amazon site preset."""
import time
import random


class Amazon:
    RATE_LIMIT = 10
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Amazon._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Amazon._last_request = time.time()

    def product(self, asin, country="US"):
        """Fetch Amazon product page by ASIN."""
        self._rate_limit()
        domain = {"US": "amazon.com", "UK": "amazon.co.uk", "DE": "amazon.de", "FR": "amazon.fr", "JP": "amazon.co.jp", "CA": "amazon.ca"}.get(country, "amazon.com")
        url = f"https://www.{domain}/dp/{asin}"
        from ..fingerprint import chrome_fingerprint
        resp = self.client.fetch(url, country=country, headers=chrome_fingerprint(country), retries=3)
        return {"asin": asin, "url": url, "status": resp.status_code, "html": resp.text, "size_kb": len(resp.text) // 1024}

    def search(self, query, country="US"):
        """Search Amazon."""
        self._rate_limit()
        import urllib.parse
        domain = {"US": "amazon.com", "UK": "amazon.co.uk", "DE": "amazon.de"}.get(country, "amazon.com")
        url = f"https://www.{domain}/s?k={urllib.parse.quote_plus(query)}"
        resp = self.client.fetch(url, country=country, retries=3)
        return {"query": query, "status": resp.status_code, "html": resp.text}

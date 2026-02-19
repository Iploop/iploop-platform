"""Google site preset."""
import time
import random


class Google:
    RATE_LIMIT = 8
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Google._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Google._last_request = time.time()

    def search(self, query, country="US", num=10, extract=False):
        """Google search. Returns HTML with results."""
        self._rate_limit()
        import urllib.parse
        q = urllib.parse.quote_plus(query)
        url = f"https://www.google.com/search?q={q}&num={num}&hl=en"
        from ..fingerprint import chrome_fingerprint
        resp = self.client.fetch(url, country=country, headers=chrome_fingerprint(country))
        result = {"query": query, "country": country, "status": resp.status_code, "html": resp.text, "size_kb": len(resp.text) // 1024}
        if extract and resp.status_code == 200:
            from .extractors import Extractors
            result["results"] = Extractors.google_results(resp.text)
        return result

    def maps(self, query, country="US"):
        """Google Maps search."""
        self._rate_limit()
        import urllib.parse
        url = f"https://www.google.com/maps/search/{urllib.parse.quote_plus(query)}"
        resp = self.client.fetch(url, country=country)
        return {"query": query, "status": resp.status_code, "html": resp.text}

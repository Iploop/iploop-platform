"""Nasdaq site preset."""
import time
import random


class Nasdaq:
    RATE_LIMIT = 8
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Nasdaq._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Nasdaq._last_request = time.time()

    def quote(self, symbol, extract=False):
        """Fetch Nasdaq stock quote page."""
        self._rate_limit()
        url = f"https://www.nasdaq.com/market-activity/stocks/{symbol.lower()}"
        resp = self.client.fetch(url, country="US")
        result = {"symbol": symbol, "status": resp.status_code, "html": resp.text}
        if extract and resp.status_code == 200:
            from .extractors import Extractors
            result.update(Extractors.nasdaq_quote(resp.text))
        return result

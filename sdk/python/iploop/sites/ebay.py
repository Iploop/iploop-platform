"""eBay site preset."""
import time
import random


class eBay:
    RATE_LIMIT = 15
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - eBay._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        eBay._last_request = time.time()

    def search(self, query, country="US", extract=False):
        """Search eBay."""
        self._rate_limit()
        import urllib.parse
        domain = {"US": "ebay.com", "UK": "ebay.co.uk", "DE": "ebay.de"}.get(country, "ebay.com")
        url = f"https://www.{domain}/sch/i.html?_nkw={urllib.parse.quote_plus(query)}"
        resp = self.client.fetch(url, country=country)
        result = {"query": query, "status": resp.status_code, "html": resp.text}
        if extract and resp.status_code == 200:
            from .extractors import Extractors
            result["products"] = Extractors.ebay_products(resp.text)
        return result

    def item(self, item_id):
        """Fetch an eBay item page."""
        self._rate_limit()
        url = f"https://www.ebay.com/itm/{item_id}"
        resp = self.client.fetch(url, country="US")
        return {"item_id": item_id, "status": resp.status_code, "html": resp.text}

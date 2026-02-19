"""Twitter site preset."""
import time
import random


class Twitter:
    RATE_LIMIT = 5
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Twitter._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Twitter._last_request = time.time()

    def _mobile_headers(self):
        from ..fingerprint import chrome_fingerprint
        return chrome_fingerprint("US")

    def profile(self, username, extract=False):
        """Fetch Twitter profile page."""
        self._rate_limit()
        url = f"https://twitter.com/{username}"
        resp = self.client.fetch(url, country="US", headers=self._mobile_headers())
        result = {"url": url, "status": resp.status_code, "html": resp.text, "size_kb": len(resp.text) // 1024}
        if extract and resp.status_code == 200:
            from .extractors import Extractors
            result.update(Extractors.twitter_profile(resp.text))
        return result

    def tweet(self, tweet_id):
        """Fetch a specific tweet."""
        self._rate_limit()
        url = f"https://twitter.com/i/status/{tweet_id}"
        resp = self.client.fetch(url, country="US", headers=self._mobile_headers())
        return {"url": url, "status": resp.status_code, "html": resp.text}

    def search(self, query):
        """Search Twitter."""
        self._rate_limit()
        url = f"https://twitter.com/search?q={query}&src=typed_query"
        resp = self.client.fetch(url, country="US", headers=self._mobile_headers())
        return {"url": url, "status": resp.status_code, "html": resp.text}

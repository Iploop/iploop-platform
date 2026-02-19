"""Reddit site preset."""
import time
import random


class Reddit:
    RATE_LIMIT = 5
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Reddit._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Reddit._last_request = time.time()

    def subreddit(self, name):
        """Fetch a subreddit page."""
        self._rate_limit()
        url = f"https://www.reddit.com/r/{name}/"
        from ..fingerprint import chrome_fingerprint
        resp = self.client.fetch(url, country="US", headers=chrome_fingerprint("US"))
        return {"subreddit": name, "status": resp.status_code, "html": resp.text}

    def post(self, url):
        """Fetch a Reddit post by URL."""
        self._rate_limit()
        resp = self.client.fetch(url, country="US")
        return {"url": url, "status": resp.status_code, "html": resp.text}

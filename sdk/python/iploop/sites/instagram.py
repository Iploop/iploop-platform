"""Instagram site preset."""
import time
import random


class Instagram:
    RATE_LIMIT = 5
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - Instagram._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        Instagram._last_request = time.time()

    def profile(self, username):
        """Fetch Instagram profile."""
        self._rate_limit()
        url = f"https://www.instagram.com/{username}/"
        from ..fingerprint import chrome_fingerprint
        resp = self.client.fetch(url, country="US", headers=chrome_fingerprint("US"))
        return {"username": username, "status": resp.status_code, "html": resp.text}

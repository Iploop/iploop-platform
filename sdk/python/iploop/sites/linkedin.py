"""LinkedIn site preset."""
import time
import random


class LinkedIn:
    RATE_LIMIT = 10
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - LinkedIn._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        LinkedIn._last_request = time.time()

    def profile(self, username, country="US"):
        """Fetch LinkedIn profile page."""
        self._rate_limit()
        url = f"https://www.linkedin.com/in/{username}/"
        from ..fingerprint import chrome_fingerprint
        resp = self.client.fetch(url, country=country, headers=chrome_fingerprint(country))
        return {"username": username, "status": resp.status_code, "html": resp.text}

    def company(self, name, country="US"):
        """Fetch LinkedIn company page."""
        self._rate_limit()
        url = f"https://www.linkedin.com/company/{name}/"
        resp = self.client.fetch(url, country=country)
        return {"company": name, "status": resp.status_code, "html": resp.text}

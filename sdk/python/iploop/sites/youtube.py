"""YouTube site preset."""
import time
import random


class YouTube:
    RATE_LIMIT = 5
    _last_request = 0

    def __init__(self, client):
        self.client = client

    def _rate_limit(self):
        elapsed = time.time() - YouTube._last_request
        if elapsed < self.RATE_LIMIT:
            time.sleep(self.RATE_LIMIT - elapsed + random.uniform(0, 2))
        YouTube._last_request = time.time()

    def video(self, video_id, extract=False):
        """Fetch a YouTube video page."""
        self._rate_limit()
        url = f"https://www.youtube.com/watch?v={video_id}"
        resp = self.client.fetch(url, country="US")
        result = {"video_id": video_id, "status": resp.status_code, "html": resp.text}
        if extract and resp.status_code == 200:
            from .extractors import Extractors
            result.update(Extractors.youtube_video(resp.text))
        return result

    def search(self, query, extract=False):
        """Search YouTube."""
        self._rate_limit()
        import urllib.parse
        url = f"https://www.youtube.com/results?search_query={urllib.parse.quote_plus(query)}"
        resp = self.client.fetch(url, country="US")
        result = {"query": query, "status": resp.status_code, "html": resp.text}
        return result

    def channel(self, channel_name):
        """Fetch a YouTube channel page."""
        self._rate_limit()
        url = f"https://www.youtube.com/@{channel_name}"
        resp = self.client.fetch(url, country="US")
        return {"channel": channel_name, "status": resp.status_code, "html": resp.text}

"""Support API client."""

import sys
import requests
from .exceptions import AuthError, QuotaExceeded


class SupportClient:
    def __init__(self, api_key, api_base):
        self.api_key = api_key
        self.api_base = api_base.rstrip("/")
        self._headers = {"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"}

    def _get(self, path):
        resp = requests.get(f"{self.api_base}{path}", headers=self._headers, timeout=15, verify=True)
        if resp.status_code == 401:
            raise AuthError("Invalid API key")
        resp.raise_for_status()
        return resp.json()

    def _post(self, path, json_data):
        resp = requests.post(f"{self.api_base}{path}", headers=self._headers, json=json_data, timeout=15, verify=True)
        if resp.status_code == 401:
            raise AuthError("Invalid API key")
        resp.raise_for_status()
        return resp.json()

    def usage(self):
        data = self._get("/api/support/diagnose")
        self._check_quota(data)
        return data

    def status(self):
        return self._get("/api/support/status")

    def ask(self, question):
        return self._post("/api/support/ask", {"question": question})

    def countries(self):
        return self._get("/api/support/countries")

    def _check_quota(self, data):
        try:
            used = data.get("used_gb", 0)
            total = used + data.get("remaining_gb", 999)
            if total > 0:
                pct = used / total * 100
                if pct >= 100:
                    raise QuotaExceeded()
                if pct >= 80:
                    print(
                        f"⚠️  IPLoop: {pct:.0f}% bandwidth used. Upgrade at https://iploop.io/pricing",
                        file=sys.stderr,
                    )
        except (TypeError, KeyError):
            pass

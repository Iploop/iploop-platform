"""Concurrent fetching — up to 25 parallel requests safely."""
import concurrent.futures
import time


class BatchFetcher:
    def __init__(self, client, max_workers=10):
        self.client = client
        self.max_workers = min(max_workers, 25)  # cap at 25 (tested safe limit)

    def fetch_all(self, urls, country=None, delay=0):
        """Fetch multiple URLs concurrently. Returns list of results."""
        results = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=self.max_workers) as executor:
            futures = {}
            for i, url in enumerate(urls):
                if delay and i > 0:
                    time.sleep(delay)
                f = executor.submit(self.client.fetch, url, country=country)
                futures[f] = url

            for future in concurrent.futures.as_completed(futures):
                url = futures[future]
                try:
                    resp = future.result()
                    results.append({"url": url, "status": resp.status_code, "size_kb": len(resp.text) // 1024, "success": True})
                except Exception as e:
                    results.append({"url": url, "error": str(e), "success": False})
        return results

    def fetch_multi_country(self, url, countries):
        """Fetch same URL from multiple countries simultaneously."""
        results = {}
        with concurrent.futures.ThreadPoolExecutor(max_workers=len(countries)) as executor:
            futures = {executor.submit(self.client.fetch, url, country=c): c for c in countries}
            for future in concurrent.futures.as_completed(futures):
                country = futures[future]
                try:
                    resp = future.result()
                    results[country] = {"status": resp.status_code, "size_kb": len(resp.text) // 1024, "html": resp.text}
                except Exception as e:
                    results[country] = {"error": str(e)}
        return results

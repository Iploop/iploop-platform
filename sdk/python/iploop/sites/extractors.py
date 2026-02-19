"""Built-in data extractors for common sites."""
import re
import json


class Extractors:
    @staticmethod
    def ebay_products(html):
        """Extract product titles and prices from eBay search results."""
        products = []
        titles = re.findall(r'<h3 class="s-item__title"[^>]*>(?:<span[^>]*>)?([^<]+)', html)
        prices = re.findall(r'<span class="s-item__price">([^<]+)</span>', html)
        for i, title in enumerate(titles):
            if title.strip() and title.strip() != "Shop on eBay":
                products.append({
                    "title": title.strip(),
                    "price": prices[i].strip() if i < len(prices) else None
                })
        return products

    @staticmethod
    def nasdaq_quote(html):
        """Extract stock price and change from Nasdaq page."""
        price_match = re.search(r'"price":"(\$[\d,.]+)"', html)
        change_match = re.search(r'"change":"([^"]+)"', html)
        pct_match = re.search(r'"pctChange":"([^"]+)"', html)
        return {
            "price": price_match.group(1) if price_match else None,
            "change": change_match.group(1) if change_match else None,
            "pct_change": pct_match.group(1) if pct_match else None
        }

    @staticmethod
    def youtube_video(html):
        """Extract video title, channel, views from YouTube page."""
        title = re.search(r'<title>([^<]+)</title>', html)
        channel = re.search(r'"ownerChannelName":"([^"]+)"', html)
        views = re.search(r'"viewCount":"(\d+)"', html)
        return {
            "title": title.group(1).replace(" - YouTube", "").strip() if title else None,
            "channel": channel.group(1) if channel else None,
            "views": int(views.group(1)) if views else None
        }

    @staticmethod
    def google_results(html):
        """Extract search results from Google SERP."""
        results = []
        blocks = re.findall(
            r'<a href="(/url\?q=|)(https?://[^"&]+)[^"]*"[^>]*>.*?<h3[^>]*>([^<]+)</h3>',
            html, re.DOTALL
        )
        for _, url, title in blocks:
            results.append({"title": title.strip(), "url": url})
        return results

    @staticmethod
    def twitter_profile(html):
        """Extract basic profile info from Twitter page."""
        name = re.search(r'<title>([^(]+)\(', html)
        handle = re.search(r'<title>[^(]+\(@(\w+)\)', html)
        return {
            "name": name.group(1).strip() if name else None,
            "handle": handle.group(1) if handle else None
        }

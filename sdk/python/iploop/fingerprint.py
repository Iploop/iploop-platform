"""Browser fingerprint headers — the universal recipe that works everywhere."""
import random

# Chrome versions pool
CHROME_VERSIONS = [
    "120.0.0.0", "121.0.0.0", "122.0.0.0", "123.0.0.0",
    "124.0.0.0", "125.0.0.0", "126.0.0.0"
]

def chrome_fingerprint(country="US"):
    """Returns full 14-header Chrome desktop fingerprint that bypasses most blocks."""
    ver = random.choice(CHROME_VERSIONS)
    lang_map = {
        "US": "en-US,en;q=0.9", "GB": "en-GB,en;q=0.9", "DE": "de-DE,de;q=0.9,en;q=0.8",
        "FR": "fr-FR,fr;q=0.9,en;q=0.8", "BR": "pt-BR,pt;q=0.9,en;q=0.8",
        "JP": "ja-JP,ja;q=0.9,en;q=0.8", "KR": "ko-KR,ko;q=0.9,en;q=0.8",
        "IN": "en-IN,en;q=0.9,hi;q=0.8", "AU": "en-AU,en;q=0.9",
        "CA": "en-CA,en;q=0.9", "IT": "it-IT,it;q=0.9,en;q=0.8",
        "ES": "es-ES,es;q=0.9,en;q=0.8", "NL": "nl-NL,nl;q=0.9,en;q=0.8",
        "PH": "en-PH,en;q=0.9", "NG": "en-NG,en;q=0.9", "ZA": "en-ZA,en;q=0.9"
    }
    lang = lang_map.get(country, "en-US,en;q=0.9")

    return {
        "User-Agent": f"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/{ver} Safari/537.36",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
        "Accept-Language": lang,
        "Accept-Encoding": "gzip, deflate, br",
        "Connection": "keep-alive",
        "Upgrade-Insecure-Requests": "1",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "none",
        "Sec-Fetch-User": "?1",
        "Sec-Ch-Ua": f'"Chromium";v="{ver.split(".")[0]}", "Google Chrome";v="{ver.split(".")[0]}", "Not-A.Brand";v="99"',
        "Sec-Ch-Ua-Mobile": "?0",
        "Sec-Ch-Ua-Platform": '"Windows"',
        "Cache-Control": "max-age=0"
    }

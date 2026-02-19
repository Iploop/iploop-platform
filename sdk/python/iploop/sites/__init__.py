"""Site-specific presets for popular websites."""

from .twitter import Twitter
from .google import Google
from .amazon import Amazon
from .instagram import Instagram
from .tiktok import TikTok
from .youtube import YouTube
from .reddit import Reddit
from .ebay import eBay
from .nasdaq import Nasdaq
from .linkedin import LinkedIn
from .extractors import Extractors

__all__ = [
    "Twitter", "Google", "Amazon", "Instagram", "TikTok",
    "YouTube", "Reddit", "eBay", "Nasdaq", "LinkedIn", "Extractors",
]

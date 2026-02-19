"""IPLoop — Residential proxy SDK."""

__version__ = "1.3.0"

from .client import IPLoop, StickySession
from .exceptions import IPLoopError, AuthError, QuotaExceeded, ProxyError, TimeoutError

try:
    from .async_client import AsyncIPLoop
except ImportError:
    AsyncIPLoop = None

__all__ = [
    "IPLoop", "AsyncIPLoop", "StickySession",
    "IPLoopError", "AuthError", "QuotaExceeded", "ProxyError", "TimeoutError",
]

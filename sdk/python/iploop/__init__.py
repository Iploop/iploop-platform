"""IPLoop — Residential proxy SDK."""

__version__ = "1.3.3"

from .client import IPLoop, StickySession
from .exceptions import IPLoopError, AuthError, QuotaExceeded, ProxyError, TimeoutError

try:
    from .async_client import AsyncIPLoop
except ImportError:
    AsyncIPLoop = None


def _check_version():
    """Non-blocking version check — warns if a newer version is available."""
    import threading

    def _check():
        try:
            import json
            from urllib.request import urlopen
            resp = urlopen("https://pypi.org/pypi/iploop/json", timeout=3)
            data = json.loads(resp.read())
            latest = data["info"]["version"]
            if latest != __version__:
                import warnings
                warnings.warn(
                    f"\n⚠️  IPLoop v{latest} available (you have {__version__}). "
                    f"Run: pip install --upgrade iploop",
                    stacklevel=2,
                )
        except Exception:
            pass  # never break user code

    threading.Thread(target=_check, daemon=True).start()


_check_version()

__all__ = [
    "IPLoop", "AsyncIPLoop", "StickySession",
    "IPLoopError", "AuthError", "QuotaExceeded", "ProxyError", "TimeoutError",
]

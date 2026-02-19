"""IPLoop SDK exceptions."""


class IPLoopError(Exception):
    """Base exception for IPLoop SDK."""


class AuthError(IPLoopError):
    """Invalid or missing API key."""


class QuotaExceeded(IPLoopError):
    """Bandwidth quota exceeded."""

    def __init__(self, message=None):
        super().__init__(
            message or "Quota exceeded. Upgrade at https://iploop.io/pricing"
        )


class ProxyError(IPLoopError):
    """Proxy connection failed after all retries."""


class TimeoutError(IPLoopError):
    """All retries timed out."""

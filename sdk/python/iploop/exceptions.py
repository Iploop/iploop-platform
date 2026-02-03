"""
IPLoop SDK Exceptions
"""


class IPLoopError(Exception):
    """Base exception for IPLoop SDK"""
    
    def __init__(self, message: str, status_code: int = None, response: dict = None):
        self.message = message
        self.status_code = status_code
        self.response = response
        super().__init__(self.message)


class AuthenticationError(IPLoopError):
    """Raised when API key is invalid or missing"""
    pass


class RateLimitError(IPLoopError):
    """Raised when rate limit is exceeded"""
    
    def __init__(self, message: str, retry_after: int = None, **kwargs):
        super().__init__(message, **kwargs)
        self.retry_after = retry_after


class QuotaExceededError(IPLoopError):
    """Raised when bandwidth or request quota is exceeded"""
    
    def __init__(self, message: str, quota_type: str = None, **kwargs):
        super().__init__(message, **kwargs)
        self.quota_type = quota_type  # "bandwidth" or "requests"


class ProxyError(IPLoopError):
    """Raised when proxy connection fails"""
    pass


class NodeUnavailableError(IPLoopError):
    """Raised when no nodes are available for the requested criteria"""
    pass

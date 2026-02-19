"""Retry logic with IP rotation."""

import time
import uuid
import logging

logger = logging.getLogger("iploop")

RETRYABLE_STATUS = {403, 407, 429, 500, 502, 503, 504}


def should_retry(exc=None, status_code=None):
    if status_code and status_code in RETRYABLE_STATUS:
        return True
    if exc:
        from requests.exceptions import (ConnectionError, Timeout, ProxyError)
        return isinstance(exc, (ConnectionError, Timeout, ProxyError))
    return False


def new_session_id():
    return uuid.uuid4().hex[:16]


def retry_request(func, retries=3, delay=1.0):
    """Execute func with retries. func receives attempt number."""
    last_exc = None
    for attempt in range(retries):
        try:
            resp = func(attempt)
            if hasattr(resp, 'status_code') and should_retry(status_code=resp.status_code):
                logger.debug("Retry %d/%d: status %d", attempt + 1, retries, resp.status_code)
                if attempt < retries - 1:
                    time.sleep(delay * (attempt + 1))
                    continue
            return resp
        except Exception as e:
            last_exc = e
            if should_retry(exc=e) and attempt < retries - 1:
                logger.debug("Retry %d/%d: %s", attempt + 1, retries, e)
                time.sleep(delay * (attempt + 1))
            elif attempt == retries - 1:
                break
            else:
                raise
    raise last_exc

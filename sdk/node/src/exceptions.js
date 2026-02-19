class IPLoopError extends Error {
  constructor(message) { super(message); this.name = 'IPLoopError'; }
}
class AuthError extends IPLoopError {
  constructor(message) { super(message || 'Invalid or missing API key'); this.name = 'AuthError'; }
}
class QuotaExceeded extends IPLoopError {
  constructor(message) { super(message || 'Quota exceeded. Upgrade at https://iploop.io/pricing'); this.name = 'QuotaExceeded'; }
}
class ProxyError extends IPLoopError {
  constructor(message) { super(message || 'Proxy connection failed'); this.name = 'ProxyError'; }
}
class TimeoutError extends IPLoopError {
  constructor(message) { super(message || 'Request timed out'); this.name = 'TimeoutError'; }
}
module.exports = { IPLoopError, AuthError, QuotaExceeded, ProxyError, TimeoutError };

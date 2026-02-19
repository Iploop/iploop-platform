class StickySession {
  constructor(client, sessionId, country, city) {
    this._client = client;
    this.sessionId = sessionId;
    this.country = country || client._country;
    this.city = city || client._city;
  }
  fetch(url, opts = {}) {
    return this._client.fetch(url, {
      country: opts.country || this.country,
      city: opts.city || this.city,
      session: this.sessionId,
      _noRotate: true,
      ...opts,
    });
  }
}
module.exports = { StickySession };

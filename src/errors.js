/**
 * Error types surfaced by the relay.
 *
 * Every error carries a stable `code` and never embeds credential material.
 */

export class RelayError extends Error {
  constructor(code, message, options = {}) {
    super(message, options);
    this.name = new.target.name;
    this.code = code;
  }
}

export class BundleError extends RelayError {
  constructor(message, options) {
    super('BUNDLE_INVALID', message, options);
  }
}

export class TooManyRedirectsError extends RelayError {
  constructor(url, limit) {
    super('TOO_MANY_REDIRECTS', `refusing to follow more than ${limit} redirects (last url: ${url})`);
    this.url = url;
    this.limit = limit;
  }
}

export class InsecureRedirectError extends RelayError {
  constructor(from, to) {
    super('INSECURE_REDIRECT', `refusing to downgrade from https to http (${from} -> ${to})`);
    this.from = from;
    this.to = to;
  }
}

export class TransportError extends RelayError {
  constructor(message, cause) {
    super('TRANSPORT_ERROR', message, { cause });
  }
}

export class AbortedError extends RelayError {
  constructor(message = 'request aborted') {
    super('ABORTED', message);
  }
}

export class OptionsError extends RelayError {
  constructor(message) {
    super('INVALID_OPTIONS', message);
  }
}

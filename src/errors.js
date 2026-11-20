'use strict';

/**
 * Error type carried out of the parsing layer.
 *
 * `status` is the HTTP status the gateway should return, and `code` is a
 * stable machine-readable reason that callers can log or map to a response.
 */
class MultipartError extends Error {
  constructor(status, code, message) {
    super(message);
    this.name = 'MultipartError';
    this.status = status;
    this.code = code;
  }

  toJSON() {
    return { status: this.status, code: this.code, message: this.message };
  }
}

module.exports = { MultipartError };

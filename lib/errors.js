'use strict';

/**
 * Error type shared by the issuing pipeline. `status` mirrors the HTTP status
 * the service returns to the caller; the CLI prints the same shape.
 */
class CertbraceError extends Error {
  constructor(message, { code = 'certbrace_error', status = 400, details } = {}) {
    super(message);
    this.name = 'CertbraceError';
    this.code = code;
    this.status = status;
    if (details !== undefined) {
      this.details = details;
    }
  }

  toJSON() {
    const body = { error: this.code, message: this.message, status: this.status };
    if (this.details !== undefined) {
      body.details = this.details;
    }
    return body;
  }
}

module.exports = { CertbraceError };

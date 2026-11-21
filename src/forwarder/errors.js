'use strict';

class SpoolError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'SpoolError';
    this.code = details.code || 'spool_error';
    this.file = details.file;
    this.offset = details.offset;
    this.cause = details.cause;
  }
}

module.exports = { SpoolError };

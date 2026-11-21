'use strict';

class SchemaError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'SchemaError';
    this.code = details.code || 'SCHEMA_ERROR';
    this.ref = details.ref || null;
    this.pointer = details.pointer || null;
    this.keyword = details.keyword || null;
  }
}

class RefError extends SchemaError {
  constructor(message, details = {}) {
    super(message, { code: 'REF_ERROR', ...details });
    this.name = 'RefError';
  }
}

module.exports = { SchemaError, RefError };
'use strict';

const CODES = Object.freeze({
  FORMAT: 'ERR_ENVELOPE_FORMAT',
  SIGNATURE: 'ERR_ENVELOPE_SIGNATURE',
  CONFIG: 'ERR_ENVELOPE_CONFIG',
});

class EnvelopeError extends Error {
  constructor(message, code, options) {
    super(message, options);
    this.name = new.target.name;
    this.code = code;
  }
}

class EnvelopeFormatError extends EnvelopeError {
  constructor(message, options) {
    super(message, CODES.FORMAT, options);
  }
}

class EnvelopeSignatureError extends EnvelopeError {
  constructor(message, options) {
    super(message, CODES.SIGNATURE, options);
  }
}

class EnvelopeConfigError extends EnvelopeError {
  constructor(message, options) {
    super(message, CODES.CONFIG, options);
  }
}

module.exports = {
  CODES,
  EnvelopeError,
  EnvelopeFormatError,
  EnvelopeSignatureError,
  EnvelopeConfigError,
};

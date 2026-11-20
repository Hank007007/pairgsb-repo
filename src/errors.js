export const ERROR_CODES = Object.freeze({
  INVALID_SUBJECT: 'INVALID_SUBJECT',
  INVALID_PURPOSE: 'INVALID_PURPOSE',
  INVALID_META: 'INVALID_META',
  INVALID_TTL: 'INVALID_TTL',
  TOKEN_MISSING: 'TOKEN_MISSING',
  TOKEN_UNKNOWN: 'TOKEN_UNKNOWN',
  TOKEN_EXPIRED: 'TOKEN_EXPIRED',
  TOKEN_ALREADY_USED: 'TOKEN_ALREADY_USED',
  TOKEN_SUBJECT_MISMATCH: 'TOKEN_SUBJECT_MISMATCH',
  TOKEN_PURPOSE_MISMATCH: 'TOKEN_PURPOSE_MISMATCH',
  STORAGE_FAILURE: 'STORAGE_FAILURE',
});

export class TokenError extends Error {
  constructor(code, message, details) {
    super(message ?? code);
    this.name = 'TokenError';
    this.code = code;
    if (details !== undefined) {
      this.details = details;
    }
  }

  toJSON() {
    return {
      name: this.name,
      code: this.code,
      message: this.message,
    };
  }
}

export function isTokenError(value) {
  return value instanceof TokenError;
}

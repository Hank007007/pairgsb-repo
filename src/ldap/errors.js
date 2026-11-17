export class LdapError extends Error {
  constructor(message, code, details = {}) {
    super(message);
    this.name = 'LdapError';
    this.code = code;
    this.details = details;
  }
}

export const ErrorCodes = Object.freeze({
  INVALID_DN_SYNTAX: 'INVALID_DN_SYNTAX',
  INVALID_SCOPE: 'INVALID_SCOPE',
  NO_SUCH_OBJECT: 'NO_SUCH_OBJECT',
  INVALID_CREDENTIALS: 'INVALID_CREDENTIALS',
  PROTOCOL_ERROR: 'PROTOCOL_ERROR',
  UNWILLING_TO_PERFORM: 'UNWILLING_TO_PERFORM',
  SIZE_LIMIT_EXCEEDED: 'SIZE_LIMIT_EXCEEDED',
});

export function invalidDn(message, details) {
  return new LdapError(message, ErrorCodes.INVALID_DN_SYNTAX, details);
}

export function protocolError(message, details) {
  return new LdapError(message, ErrorCodes.PROTOCOL_ERROR, details);
}

/**
 * Error type used across the service.
 *
 * Every failure that is part of the service contract is raised as an
 * `AuthError` carrying a stable `code`. Callers (HTTP layer, CLI) translate
 * the code into a status code; the evaluator itself never turns an error into
 * a boolean.
 */

export const ErrorCodes = Object.freeze({
  NAMESPACE_NOT_FOUND: 'namespace_not_found',
  INVALID_NAMESPACE: 'invalid_namespace',
  INVALID_TUPLE: 'invalid_tuple',
  INVALID_REQUEST: 'invalid_request',
  CYCLE_DETECTED: 'cycle_detected',
  DEPTH_EXCEEDED: 'depth_exceeded',
  TENANT_MISMATCH: 'tenant_mismatch',
  TUPLE_STORE_ERROR: 'tuple_store_error',
});

export class AuthError extends Error {
  /**
   * @param {string} code one of ErrorCodes
   * @param {string} message human readable, safe to log
   * @param {object} [details] structured context (namespace, relation, object)
   */
  constructor(code, message, details = {}) {
    super(message);
    this.name = 'AuthError';
    this.code = code;
    this.details = details;
  }

  toJSON() {
    return { error: this.code, message: this.message, details: this.details };
  }
}

export function isAuthError(value) {
  return value instanceof AuthError;
}

export function fail(code, message, details) {
  throw new AuthError(code, message, details);
}

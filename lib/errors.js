/**
 * Shared error type for protolens.
 *
 * Every failure that crosses a layer boundary is an instance of
 * ProtolensError carrying a stable `code` string. Callers branch on `code`,
 * never on the message text.
 */
export class ProtolensError extends Error {
  /**
   * @param {string} code stable machine-readable code
   * @param {string} message human-readable description
   * @param {object} [details] optional extra properties (offset, field, ...)
   */
  constructor(code, message, details = {}) {
    super(message);
    this.name = 'ProtolensError';
    this.code = code;
    Object.assign(this, details);
  }
}

export const CODES = Object.freeze({
  VARINT_TOO_LONG: 'varint_too_long',
  VARINT_NOT_MINIMAL: 'varint_not_minimal',
  VARINT_TRUNCATED: 'varint_truncated',
  VARINT_RANGE: 'varint_range',
  INVALID_FIELD_NUMBER: 'invalid_field_number',
  INVALID_WIRE_TYPE: 'invalid_wire_type',
  WIRE_TYPE_MISMATCH: 'wire_type_mismatch',
  LENGTH_OUT_OF_RANGE: 'length_out_of_range',
  TRUNCATED_FIELD: 'truncated_field',
  MAX_DEPTH_EXCEEDED: 'max_depth_exceeded',
  SIZE_LIMIT_EXCEEDED: 'size_limit_exceeded',
  SCHEMA_NOT_FOUND: 'schema_not_found',
  SIGNATURE_INVALID: 'signature_invalid',
  BODY_TOO_LARGE: 'body_too_large',
  DECODE_FAILED: 'decode_failed',
});

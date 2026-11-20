'use strict';

/**
 * Error type shared by the lexer, parser and compiler. Every rejection carries
 * a stable machine-readable code so callers can distinguish user mistakes
 * from internal failures.
 */
class RuleError extends Error {
  constructor(code, message, position = null) {
    const suffix = Number.isInteger(position) ? ` (at position ${position})` : '';
    super(`${message}${suffix}`);
    this.name = 'RuleError';
    this.code = code;
    this.position = position;
  }
}

function isRuleError(value) {
  return value instanceof RuleError;
}

module.exports = {
  RuleError,
  isRuleError,
};

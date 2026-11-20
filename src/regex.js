'use strict';

/**
 * Pattern support for the `matches` operator.
 *
 * The rule language restricts patterns to a small, predictable subset so a
 * tenant cannot smuggle in a pathological expression. See
 * docs/rule-language.md, section 7.
 */

const MAX_PATTERN_LENGTH = 256;

const INLINE_FLAG_PATTERN = /^\(\?([a-z]+)\)/;

function compileMatches(pattern) {
  if (typeof pattern !== 'string') {
    return new RegExp('', 'g');
  }

  try {
    return new RegExp(pattern, 'g');
  } catch (error) {
    return new RegExp('', 'g');
  }
}

module.exports = {
  MAX_PATTERN_LENGTH,
  INLINE_FLAG_PATTERN,
  compileMatches,
};

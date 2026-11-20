'use strict';

const { RuleError } = require('./errors');

/**
 * Pattern support for the `matches` operator.
 *
 * The rule language restricts patterns to a small, predictable subset so a
 * tenant cannot smuggle in a pathological expression. See
 * docs/rule-language.md, section 6.5.
 */

const MAX_PATTERN_LENGTH = 256;

const INLINE_FLAG_PATTERN = /^\(\?([ism])\)/;
const FLAG_ORDER = 'ism';
const QUANTIFIER_PATTERN = /^\{\d+(,\d*)?\}/;

function badPattern(reason, position) {
  return new RuleError('E_COMPILE_BAD_PATTERN', `Pattern is outside the allowed subset: ${reason}`, position);
}

function isAlphanumeric(ch) {
  return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9');
}

function scanCharacterClass(body, start) {
  let i = start + 1;
  if (body[i] === '^') i += 1;
  if (body[i] === ']') i += 1;
  while (i < body.length && body[i] !== ']') {
    if (body[i] === '\\') {
      const escaped = body[i + 1];
      if (escaped === undefined || isAlphanumeric(escaped)) {
        throw badPattern('only punctuation escapes are allowed', i);
      }
      i += 2;
      continue;
    }
    i += 1;
  }
  if (i >= body.length) {
    throw badPattern('unterminated character class', start);
  }
  return i + 1;
}

function validateBody(body, offset) {
  let i = 0;
  while (i < body.length) {
    const ch = body[i];
    if (ch === '\\') {
      const escaped = body[i + 1];
      if (escaped === undefined || isAlphanumeric(escaped)) {
        throw badPattern('only escapes of a single non-alphanumeric character are allowed', offset + i);
      }
      i += 2;
      continue;
    }
    if (ch === '[') {
      i = scanCharacterClass(body, i);
      continue;
    }
    if (ch === '(') {
      if (body.startsWith('(?', i)) {
        if (!body.startsWith('(?:', i)) {
          throw badPattern('only non-capturing groups "(?:...)" are allowed', offset + i);
        }
        i += 3;
        continue;
      }
      i += 1;
      continue;
    }
    if (ch === '{') {
      const quantifier = QUANTIFIER_PATTERN.exec(body.slice(i));
      if (!quantifier) {
        throw badPattern('"{}" is only allowed as a {n}, {n,} or {n,m} quantifier', offset + i);
      }
      i += quantifier[0].length;
      continue;
    }
    i += 1;
  }
}

/**
 * Validate a pattern against the allowed subset. Returns `{ flags, body }`
 * where `flags` is a string of JS RegExp flags. Throws a RuleError with code
 * E_COMPILE_BAD_PATTERN when the pattern is not in the subset.
 */
function parsePattern(pattern) {
  if (typeof pattern !== 'string') {
    throw badPattern('pattern must be a string', null);
  }
  if (pattern.length > MAX_PATTERN_LENGTH) {
    throw badPattern(`pattern is longer than ${MAX_PATTERN_LENGTH} characters`, 0);
  }

  let offset = 0;
  let flags = '';
  while (pattern.startsWith('(?', offset)) {
    const inline = INLINE_FLAG_PATTERN.exec(pattern.slice(offset));
    if (!inline) {
      throw badPattern('only the inline flags (?i), (?s) and (?m) are allowed', offset);
    }
    const flag = inline[1];
    if (flags.includes(flag) || flag <= flags.at(-1)) {
      throw badPattern('inline flags must appear at most once, in the order i, s, m', offset);
    }
    flags += flag;
    offset += inline[0].length;
  }

  const body = pattern.slice(offset);
  validateBody(body, offset);

  try {
    // eslint-disable-next-line no-new
    new RegExp(body, flags);
  } catch (error) {
    throw badPattern(error.message, offset);
  }

  return { flags, body };
}

function compileMatches(pattern) {
  try {
    const { flags, body } = parsePattern(pattern);
    return new RegExp(body, flags);
  } catch (error) {
    return null;
  }
}

module.exports = {
  MAX_PATTERN_LENGTH,
  parsePattern,
  compileMatches,
};

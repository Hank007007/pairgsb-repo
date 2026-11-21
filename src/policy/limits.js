'use strict';

/**
 * Process-wide hard limits and policy constants.
 *
 * These values are intentionally plain exports so that tests and the HTTP
 * layer can reason about them without constructing the whole gateway.
 */

const maxBodyBytes = 1024 * 1024;

const maxPatternLength = 512;

const maxDepth = 32;

const maxRefDepth = 4;

const fetchTimeoutMs = 2000;

const maxRemoteBytes = 64 * 1024;

const remoteHostAllowlist = Object.freeze([
  'schemas.example.com',
]);

const dialectVersion = '2026-01';

/**
 * Heuristic check for regular-expression constructs that the gateway policy
 * does not allow. Accepts a pattern source and, optionally, flags.
 *
 * @param {string} source
 * @param {string} [flags]
 * @returns {{safe: boolean, reason: string|null}}
 */
function isRegexSafe(source, flags) {
  if (typeof source !== 'string') {
    return { safe: false, reason: 'pattern must be a string' };
  }
  if (Buffer.byteLength(source, 'utf8') > maxPatternLength) {
    return { safe: false, reason: 'pattern exceeds maximum length' };
  }

  const forbidden = [
    [/\\[1-9]/, 'backreference'],
    [/\\k</, 'named backreference'],
    [/\(\?<(?![=!])/, 'named capture group'],
    [/\(\?[=!]/, 'lookahead assertion'],
    [/\(\?<[=!]/, 'lookbehind assertion'],
    [/\(\?>/, 'atomic group'],
    [/\(\?(?:R|0|\d+)\)/, 'recursion'],
    [/\(\?&/, 'subroutine call'],
    [/\(\?[imsx]+\)/, 'inline modifier group'],
    [/\[\^?[^\]]*\[/, 'nested character class'],
  ];

  for (const [re, label] of forbidden) {
    if (re.test(source)) {
      return { safe: false, reason: `forbidden regex construct: ${label}` };
    }
  }

  if (flags !== undefined && /[^gimsuy]/.test(flags)) {
    return { safe: false, reason: 'unsupported regex flag' };
  }

  return { safe: true, reason: null };
}

module.exports = {
  maxBodyBytes,
  maxPatternLength,
  maxDepth,
  maxRefDepth,
  fetchTimeoutMs,
  maxRemoteBytes,
  remoteHostAllowlist,
  dialectVersion,
  isRegexSafe,
};
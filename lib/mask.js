'use strict';

// Log masking primitives.
//
// `buildMatcher` indexes a set of secret values for scanning a byte stream.
// `mask` rewrites a piece of text, replacing every occurrence of a known
// secret with a fixed placeholder.

const PLACEHOLDER = '[REDACTED]';

function buildMatcher(secrets) {
  const unique = [];
  const seen = new Set();

  for (const secret of secrets || []) {
    if (typeof secret !== 'string') continue;
    if (secret.length === 0) continue;
    if (seen.has(secret)) continue;
    seen.add(secret);
    unique.push(secret);
  }

  // Longest first so a shorter prefix cannot consume the head of a longer
  // secret before it gets a chance to match.
  unique.sort((a, b) => b.length - a.length || (a < b ? -1 : 1));

  const patterns = unique.map((secret) => Buffer.from(secret, 'utf8'));

  function matchAt(buffer, position) {
    for (let i = 0; i < patterns.length; i += 1) {
      const pattern = patterns[i];
      if (position + pattern.length > buffer.length) continue;
      if (buffer.compare(pattern, 0, pattern.length, position, position + pattern.length) === 0) {
        return { secret: unique[i], length: pattern.length };
      }
    }
    return null;
  }

  // Length of the longest suffix of `buffer` that is still a proper prefix of
  // some secret. A streaming caller must keep at least this many trailing
  // bytes before deciding they cannot start a match.
  function holdbackLength(buffer) {
    let best = 0;

    for (const pattern of patterns) {
      const limit = Math.min(pattern.length - 1, buffer.length);
      for (let length = limit; length > best; length -= 1) {
        if (buffer.compare(pattern, 0, length, buffer.length - length, buffer.length) === 0) {
          best = length;
          break;
        }
      }
    }

    return best;
  }

  return {
    secrets: unique.slice(),
    patterns,
    maxLength: patterns.length > 0 ? patterns[0].length : 0,
    matchAt,
    holdbackLength,
  };
}

function mask(secrets, text) {
  let output = String(text);

  for (const secret of secrets || []) {
    if (secret === '') return '';
    if (typeof secret !== 'string') continue;
    output = output.split(secret).join(PLACEHOLDER);
  }

  return output;
}

module.exports = {
  PLACEHOLDER,
  buildMatcher,
  mask,
};

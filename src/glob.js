const REGEX_SPECIAL = new Set([
  '\\',
  '^',
  '$',
  '.',
  '|',
  '?',
  '+',
  '(',
  ')',
  '[',
  ']',
  '{',
  '}',
]);

function escapeLiteral(character) {
  return REGEX_SPECIAL.has(character) ? `\\${character}` : character;
}

/**
 * Compile a path-style glob into an anchored regular expression.
 *
 * @param {string} pattern
 * @returns {RegExp}
 */
export function compileGlob(pattern) {
  if (typeof pattern !== 'string' || pattern.length === 0) {
    throw new TypeError('glob pattern must be a non-empty string');
  }

  let source = '';
  for (const character of pattern) {
    if (character === '*') {
      source += '[^]*';
    } else {
      source += escapeLiteral(character);
    }
  }

  return new RegExp(`^${source}$`);
}

/**
 * Test a value against a glob pattern.
 *
 * @param {string} pattern
 * @param {string} value
 * @returns {boolean}
 */
export function matchGlob(pattern, value) {
  return compileGlob(pattern).test(value);
}

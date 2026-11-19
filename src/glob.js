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

function compileSegment(segment) {
  if (segment === '') {
    return '';
  }
  if (segment === '*') {
    return '[^/]+';
  }

  let source = '';
  for (const character of segment) {
    source += character === '*' ? '[^/]*' : escapeLiteral(character);
  }
  return source;
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

  const absolute = pattern.startsWith('/');
  const rest = absolute ? pattern.slice(1) : pattern;
  const segments = rest === '' ? [] : rest.split('/');

  let source = absolute ? '^/' : '^';
  let emitted = false;
  let separatorConsumed = false;

  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    const isLast = index === segments.length - 1;

    if (segment === '**') {
      if (!emitted) {
        if (isLast) {
          source += '.*';
        } else {
          source += '(?:[^/]+/)*';
          separatorConsumed = true;
        }
      } else {
        source += '(?:/[^/]+)*';
      }
      emitted = true;
      continue;
    }

    const matcher = compileSegment(segment);
    if (emitted && !separatorConsumed) {
      source += '/';
    }
    source += matcher;
    emitted = true;
    separatorConsumed = false;
  }

  return new RegExp(`${source}$`);
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

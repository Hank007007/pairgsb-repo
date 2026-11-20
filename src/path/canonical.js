/**
 * Convert a path to the canonical form used for comparisons.
 *
 * @param {string} path
 * @returns {string}
 */
export function normalize(path) {
  if (typeof path !== 'string' || path.length === 0) return '/';

  let result = path.startsWith('/') ? path : `/${path}`;
  result = result.replace(/\/+/g, '/');
  if (result.length > 1 && result.endsWith('/')) {
    result = result.slice(0, -1);
  }
  return result || '/';
}

/**
 * Whether `candidate` lies at or below `ancestor`.
 *
 * @param {string} ancestor
 * @param {string} candidate
 * @returns {boolean}
 */
export function isInside(ancestor, candidate) {
  const parent = normalize(ancestor);
  const child = normalize(candidate);
  return child.startsWith(parent);
}

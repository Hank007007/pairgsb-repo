/**
 * Convert a path to the canonical form used for comparisons.
 *
 * @param {string} path
 * @returns {string}
 */
export function normalize(path) {
  if (typeof path !== 'string' || path.length === 0) return '/';

  const segments = [];
  for (const segment of path.split('/')) {
    if (segment === '' || segment === '.') continue;
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }

  return segments.length === 0 ? '/' : `/${segments.join('/')}`;
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
  if (parent === '/') return true;
  return child === parent || child.startsWith(`${parent}/`);
}

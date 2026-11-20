'use strict';

class PathError extends Error {
  constructor(message) {
    super(message);
    this.name = 'PathError';
  }
}

/**
 * Decode one path segment, rejecting malformed or separator-introducing
 * percent-encoding.
 *
 * @param {string} segment
 * @returns {string}
 */
function decodeSegment(segment) {
  let decoded;
  try {
    decoded = decodeURIComponent(segment);
  } catch {
    throw new PathError(`path segment is not valid percent-encoding: ${segment}`);
  }
  if (decoded.includes('/') || decoded.includes('\\')) {
    throw new PathError(`path segment decodes to a separator: ${segment}`);
  }
  return decoded;
}

/**
 * Reduce a path to the form used for storage and comparison.
 *
 * Backslashes are treated as slashes, percent-encoded segments are decoded,
 * empty and `.` segments are dropped, and `..` cancels the previous segment.
 * A `..` with nothing left to cancel is an escape attempt and is rejected.
 * The absolute/relative character of the input is preserved.
 *
 * @param {string} input
 * @returns {string}
 */
function normalize(input) {
  if (typeof input !== 'string') {
    throw new TypeError('path must be a string');
  }
  const replaced = input.replace(/\\/g, '/');
  const absolute = replaced.startsWith('/');
  const segments = [];
  for (const raw of replaced.split('/')) {
    if (raw === '') {
      continue;
    }
    const segment = decodeSegment(raw);
    if (segment === '' || segment === '.') {
      continue;
    }
    if (segment === '..') {
      if (segments.length === 0) {
        throw new PathError(`path climbs above its root: ${input}`);
      }
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  const joined = segments.join('/');
  return absolute ? `/${joined}` : joined;
}

/**
 * Normalize `input`, resolving a relative `input` against `root` first.
 *
 * @param {string} root
 * @param {string} input
 * @returns {string}
 */
function resolveAgainst(root, input) {
  if (typeof input !== 'string') {
    throw new TypeError('path must be a string');
  }
  const absolute = input.startsWith('/') || input.startsWith('\\');
  if (absolute) {
    return normalize(input);
  }
  return normalize(`${normalize(root)}/${input}`);
}

/**
 * Report whether `candidate` is located under `root`.
 *
 * Comparison is by path segment, not by string prefix: `/tenant-a` contains
 * `/tenant-a` itself and `/tenant-a/x`, but not `/tenant-ab/x`. A relative
 * `candidate` is resolved against `root` before the comparison.
 *
 * @param {string} root
 * @param {string} candidate
 * @returns {boolean}
 */
function isWithin(root, candidate) {
  if (typeof root !== 'string' || typeof candidate !== 'string') {
    throw new TypeError('root and candidate must be strings');
  }
  const normRoot = normalize(root);
  const normCandidate = resolveAgainst(normRoot, candidate);
  const rootSegments = normRoot.split('/').filter((segment) => segment.length > 0);
  const candidateSegments = normCandidate.split('/').filter((segment) => segment.length > 0);
  if (candidateSegments.length < rootSegments.length) {
    return false;
  }
  return rootSegments.every((segment, index) => candidateSegments[index] === segment);
}

module.exports = { PathError, normalize, resolveAgainst, isWithin };

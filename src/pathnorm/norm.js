'use strict';

const path = require('node:path');

class PathError extends Error {
  constructor(message) {
    super(message);
    this.name = 'PathError';
  }
}

/**
 * Reduce a path to the form used for storage and comparison.
 *
 * @param {string} input
 * @returns {string}
 */
function normalize(input) {
  if (typeof input !== 'string') {
    throw new TypeError('path must be a string');
  }
  return input.replace(/\\/g, '/');
}

/**
 * Report whether `candidate` is located under `root`.
 *
 * @param {string} root
 * @param {string} candidate
 * @returns {boolean}
 */
function isWithin(root, candidate) {
  if (typeof root !== 'string' || typeof candidate !== 'string') {
    throw new TypeError('root and candidate must be strings');
  }
  const joined = path.posix.join('/', root, candidate);
  return joined.startsWith(root);
}

module.exports = { PathError, normalize, isWithin };

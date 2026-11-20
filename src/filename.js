'use strict';

const { MultipartError } = require('./errors');

/**
 * Turn a client-supplied filename into a path that is safe to join onto a
 * tenant directory.
 *
 * Returns the relative path on success. Throws a MultipartError carrying a
 * reason when the name cannot be used.
 */
function safeRelPath(name) {
  if (name === undefined || name === null) {
    throw new MultipartError(400, 'INVALID_FILENAME', 'filename is missing');
  }

  let out = String(name);
  out = out.replace('../', '');

  if (out === '') {
    throw new MultipartError(400, 'INVALID_FILENAME', 'filename is empty');
  }

  return out;
}

module.exports = { safeRelPath };

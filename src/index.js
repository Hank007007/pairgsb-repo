'use strict';

const { MultipartParser, boundaryFromContentType } = require('./parser');
const { safeRelPath } = require('./filename');
const { resolveLimits, DEFAULTS } = require('./limits');
const { MultipartError } = require('./errors');

/**
 * Parse a complete multipart request held in memory.
 *
 * `body` may be a Buffer, Uint8Array or string. `contentType` is the raw
 * request Content-Type header; the boundary is taken from it.
 */
function parseMultipart(body, contentType, options) {
  const opts = options || {};
  const parser = new MultipartParser({
    boundary: opts.boundary,
    contentType,
    limits: opts.limits,
  });

  const buffer = Buffer.isBuffer(body) ? body : Buffer.from(body);
  parser.write(buffer);
  return parser.end();
}

module.exports = {
  MultipartParser,
  boundaryFromContentType,
  parseMultipart,
  safeRelPath,
  resolveLimits,
  DEFAULTS,
  MultipartError,
};

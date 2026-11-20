'use strict';

const { MultipartError } = require('./errors');

/**
 * Parse the header block of a single part.
 *
 * The block arrives without its terminating CRLFCRLF. Names are normalised to
 * lower case; repeated names keep the last occurrence.
 */
function parsePartHeaders(raw) {
  const headers = Object.create(null);
  const lines = String(raw).split('\n');

  for (const line of lines) {
    const text = line.trimEnd();
    if (text === '') continue;

    const colon = text.indexOf(':');
    if (colon <= 0) continue;

    const name = text.slice(0, colon).trim().toLowerCase();
    const value = text.slice(colon + 1).trim();
    headers[name] = value;
  }

  return headers;
}

/**
 * Parse a Content-Disposition value into its disposition type and parameters.
 *
 * Returns `{ type, params }`. Parameter names are lower case. A parameter
 * value wrapped in double quotes is returned without the quotes.
 */
function parseDispositionParams(value) {
  const text = String(value == null ? '' : value).trim();
  const segments = text.split(';');
  const type = (segments.shift() || '').trim().toLowerCase();

  if (type === '') {
    throw new MultipartError(
      400,
      'MISSING_DISPOSITION',
      'part is missing a Content-Disposition value'
    );
  }
  if (type !== 'form-data' && type !== 'file') {
    throw new MultipartError(
      400,
      'UNSUPPORTED_DISPOSITION',
      `unsupported disposition type: ${type}`
    );
  }

  const params = Object.create(null);

  for (const segment of segments) {
    const parts = segment.split('=');
    if (parts.length < 2) continue;

    const key = parts[0].trim().toLowerCase();
    let val = parts[1].trim();

    if (val.length >= 2 && val.startsWith('"') && val.endsWith('"')) {
      val = val.slice(1, -1);
    }

    if (key !== '') params[key] = val;
  }

  return { type, params };
}

module.exports = { parsePartHeaders, parseDispositionParams };

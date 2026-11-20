'use strict';

/**
 * Dialect selection for uploaded spreadsheet exports.
 *
 * `sniff` takes the raw upload plus the declared context (filename and media
 * type) and decides which delimiter the records use.
 */

const CANDIDATES = [',', '\t', ';'];

function toText(input) {
  if (typeof input === 'string') {
    return input;
  }
  if (Buffer.isBuffer(input)) {
    return input.toString('utf8');
  }
  if (ArrayBuffer.isView(input)) {
    return Buffer.from(input.buffer, input.byteOffset, input.byteLength).toString('utf8');
  }
  return String(input);
}

function hasBom(text) {
  return text.charCodeAt(0) === 0xfeff;
}

/**
 * Delimiter implied by the filename extension or media type.
 */
function declaredDelimiter(filename, contentType) {
  const name = typeof filename === 'string' ? filename.toLowerCase() : '';
  const type = typeof contentType === 'string' ? contentType.toLowerCase() : '';

  if (name.endsWith('.tsv') || type.includes('tab-separated-values')) {
    return '\t';
  }
  if (name.endsWith('.csv') || type.includes('text/csv')) {
    return ',';
  }
  return null;
}

/**
 * Pick the delimiter used by the upload.
 *
 * @param {Buffer | Uint8Array | string} input
 * @param {{ filename?: string, contentType?: string }} [options]
 */
function sniff(input, options = {}) {
  const text = toText(input);
  const declared = declaredDelimiter(options.filename, options.contentType);

  const counts = { ',': 0, '\t': 0, ';': 0 };
  for (const ch of text) {
    if (Object.prototype.hasOwnProperty.call(counts, ch)) {
      counts[ch] += 1;
    }
  }

  let delimiter = ',';
  let best = -1;
  for (const candidate of CANDIDATES) {
    if (counts[candidate] > best) {
      best = counts[candidate];
      delimiter = candidate;
    }
  }

  return {
    text,
    delimiter,
    declared,
    bom: hasBom(text),
    hint: null,
    counts,
    error: null,
  };
}

module.exports = {
  CANDIDATES,
  declaredDelimiter,
  sniff,
  toText,
};

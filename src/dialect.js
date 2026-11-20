'use strict';

/**
 * Dialect selection for uploaded spreadsheet exports.
 *
 * `sniff` takes the raw upload plus the declared context (filename and media
 * type) and decides which delimiter the records use.
 */

const { decodeText } = require('./tokenizer');

const CANDIDATES = [',', '\t', ';'];
const SAMPLE_RECORDS = 10;
const BOM = '\uFEFF';

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
 * Match and consume a leading `sep=<char>` spreadsheet hint (§2.3.2). Only the
 * candidate delimiters are accepted as a hint; anything else stays ordinary
 * data. Returns the remaining text and the hinted delimiter, if any.
 */
function consumeHint(text) {
  const match = /^[ \t]*sep=[ \t]*(.)[ \t]*(?=\r\n|\r|\n)/i.exec(text);
  if (!match) {
    return { text, hint: null };
  }

  const separator = match[1];
  if (!CANDIDATES.includes(separator)) {
    return { text, hint: null };
  }

  let end = match[0].length;
  if (text.charAt(end) === '\r' && text.charAt(end + 1) === '\n') {
    end += 2;
  } else {
    end += 1;
  }

  return { text: text.slice(end), hint: separator };
}

/**
 * How decisively a candidate delimiter explains the sampled records. A
 * candidate fits when every sampled record splits into the same number of
 * fields and that number is greater than one (§3.4).
 */
function scoreCandidate(text, delimiter) {
  const decoded = decodeText(text, delimiter);
  if (decoded.error) {
    return null;
  }

  const sample = decoded.records.slice(0, SAMPLE_RECORDS);
  if (sample.length === 0) {
    return null;
  }

  const fieldCount = sample[0].length;
  if (fieldCount <= 1) {
    return null;
  }
  if (sample.some((record) => record.length !== fieldCount)) {
    return null;
  }

  return fieldCount;
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
  const rawText = toText(input);
  const bom = hasBom(rawText);
  const withoutBom = bom ? rawText.slice(BOM.length) : rawText;
  const { text, hint } = consumeHint(withoutBom);
  const declared = declaredDelimiter(options.filename, options.contentType);

  if (hint) {
    return {
      text,
      delimiter: hint,
      declared,
      bom,
      hint,
      error: null,
    };
  }

  const fits = [];
  for (const candidate of CANDIDATES) {
    const fieldCount = scoreCandidate(text, candidate);
    if (fieldCount !== null) {
      fits.push({ delimiter: candidate, fieldCount });
    }
  }

  if (declared && fits.some((fit) => fit.delimiter === declared)) {
    return {
      text,
      delimiter: declared,
      declared,
      bom,
      hint,
      error: null,
    };
  }

  if (fits.length > 0) {
    fits.sort((a, b) => b.fieldCount - a.fieldCount);
    return {
      text,
      delimiter: fits[0].delimiter,
      declared,
      bom,
      hint,
      error: null,
    };
  }

  if (declared) {
    return {
      text,
      delimiter: declared,
      declared,
      bom,
      hint,
      error: null,
    };
  }

  return {
    text,
    delimiter: CANDIDATES[0],
    declared,
    bom,
    hint,
    error: {
      code: 'fatal_undecodable_table',
      message:
        'no candidate delimiter produces a consistent multi-field table',
      row: 0,
      column: 0,
    },
  };
}

module.exports = {
  CANDIDATES,
  declaredDelimiter,
  consumeHint,
  hasBom,
  sniff,
  toText,
};

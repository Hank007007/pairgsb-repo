'use strict';

/**
 * Entry point for the batch import service.
 *
 * `parseSheet` takes the raw upload and returns the validated header plus the
 * coerced data rows. Errors are returned next to the rows as an object with a
 * `code`, a `message`, and a `row`/`column` position.
 */

const { sniff, toText } = require('./dialect');
const { createTokenizer } = require('./tokenizer');
const { coerce } = require('./cells');
const { check } = require('./limits');

/**
 * Resolve the declared type for a column.
 *
 * @param {string} name
 * @param {Record<string, string> | undefined} types
 */
function typeOf(name, types) {
  if (!types || !Object.prototype.hasOwnProperty.call(types, name)) {
    return 'text';
  }
  return types[name];
}

/**
 * Build a positioned error.
 */
function makeError(code, message, row, column) {
  return { code, message, row, column };
}

function inputByteLength(input, text) {
  if (Buffer.isBuffer(input)) {
    return input.length;
  }
  if (ArrayBuffer.isView(input)) {
    return input.byteLength;
  }
  return Buffer.byteLength(text, 'utf8');
}

/**
 * Trim the padding a spreadsheet adds around a header cell. Only spaces and
 * tabs are padding here; other characters are part of the name.
 *
 * @param {string} name
 */
function trimHeader(name) {
  return name.replace(/^[ \t]+/, '').replace(/[ \t]+$/, '');
}

/**
 * Parse an uploaded sheet.
 *
 * @param {Buffer | Uint8Array | string} input
 * @param {{
 *   filename?: string,
 *   contentType?: string,
 *   types?: Record<string, 'text' | 'number' | 'date'>,
 * }} [options]
 */
function parseSheet(input, options = {}) {
  try {
    return parseSheetUnsafe(input, options);
  } catch (failure) {
    const code =
      typeof failure.code === 'string' && failure.code.startsWith('fatal')
        ? failure.code
        : 'fatal_uncaught_error';
    return {
      header: null,
      rows: [],
      error: makeError(
        code,
        failure.message || 'an unexpected error stopped decoding',
        failure.row || 0,
        failure.column || 0,
      ),
    };
  }
}

/**
 * @param {Buffer | Uint8Array | string} input
 * @param {{
 *   filename?: string,
 *   contentType?: string,
 *   types?: Record<string, 'text' | 'number' | 'date'>,
 * }} [options]
 */
function parseSheetUnsafe(input, options = {}) {
  const declaredTypes = options.types || {};
  const text = toText(input);
  const bytes = inputByteLength(input, text);

  const bodyError = check({ bytes });
  if (bodyError) {
    return { header: null, rows: [], error: bodyError };
  }

  const dialect = sniff(input, options);
  if (dialect.error) {
    return { header: null, rows: [], error: dialect.error };
  }

  const tokenizer = createTokenizer(dialect.text, dialect.delimiter);

  const firstRecord = tokenizer.nextRecord();
  if (tokenizer.error) {
    return { header: null, rows: [], error: tokenizer.error };
  }

  if (firstRecord === null) {
    return {
      header: null,
      rows: [],
      error: makeError('fatal_empty_input', 'the upload has no header row', 0, 0),
    };
  }

  const headerLimitError = check({
    bytes,
    fields: firstRecord,
    rowNumber: 1,
  });
  if (headerLimitError) {
    return { header: null, rows: [], error: headerLimitError };
  }

  const header = firstRecord.map((name) => trimHeader(name));

  for (let i = 0; i < header.length; i += 1) {
    if (header[i] === '') {
      return {
        header: null,
        rows: [],
        error: makeError(
          'fatal_empty_header',
          `column ${i + 1} has an empty header name`,
          1,
          i + 1,
        ),
      };
    }
  }

  const seen = new Map();
  for (let i = 0; i < header.length; i += 1) {
    if (seen.has(header[i])) {
      return {
        header: null,
        rows: [],
        error: makeError(
          'fatal_duplicate_header',
          `duplicate header name ${JSON.stringify(header[i])} at columns ${seen.get(header[i])} and ${i + 1}`,
          1,
          i + 1,
        ),
      };
    }
    seen.set(header[i], i + 1);
  }

  const rows = [];
  let record = tokenizer.nextRecord();

  if (tokenizer.error) {
    return { header: null, rows: [], error: tokenizer.error };
  }

  while (record !== null) {
    const rowNumber = tokenizer.recordNumber;

    const recordLimitError = check({
      bytes,
      fields: record,
      rowNumber,
      recordCount: rows.length + 1,
    });
    if (recordLimitError) {
      return { header, rows: [], error: recordLimitError };
    }

    if (record.length !== header.length) {
      return {
        header,
        rows: [],
        error: makeError(
          'fatal_field_count_mismatch',
          `record ${rowNumber} has ${record.length} fields; expected ${header.length}`,
          rowNumber,
          1,
        ),
      };
    }

    const row = {};
    for (let i = 0; i < header.length; i += 1) {
      try {
        row[header[i]] = coerce(record[i], typeOf(header[i], declaredTypes));
      } catch (failure) {
        return {
          header,
          rows: [],
          error: makeError(
            failure.code || 'fatal_invalid_cell',
            failure.message,
            rowNumber,
            i + 1,
          ),
        };
      }
    }

    rows.push(row);

    record = tokenizer.nextRecord();

    if (tokenizer.error) {
      return { header: null, rows: [], error: tokenizer.error };
    }
  }

  return { header, rows, error: null };
}

module.exports = {
  parseSheet,
};

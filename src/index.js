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
  const declaredTypes = options.types || {};
  const text = toText(input);
  const bytes = Buffer.byteLength(text, 'utf8');

  const bodyError = check({ bytes });
  if (bodyError) {
    return { header: null, rows: [], error: bodyError };
  }

  const dialect = sniff(input, options);
  const tokenizer = createTokenizer(dialect.text, dialect.delimiter);

  const firstRecord = tokenizer.nextRecord();
  if (firstRecord === null) {
    return {
      header: null,
      rows: [],
      error: makeError('fatal_empty_input', 'the upload has no header row', 0, 0),
    };
  }

  const header = firstRecord.map((name) => trimHeader(name));
  const rows = [];

  const headerError = check({ bytes, header });
  if (headerError) {
    return { header: null, rows, error: headerError };
  }

  let record = tokenizer.nextRecord();
  while (record !== null) {
    const rowNumber = tokenizer.recordNumber;

    if (record.length !== header.length) {
      return {
        header,
        rows,
        error: makeError(
          'field_count_mismatch',
          `record ${rowNumber} has ${record.length} fields; expected ${header.length}`,
          rowNumber,
          1,
        ),
      };
    }

    const row = {};
    for (let i = 0; i < header.length; i += 1) {
      row[header[i]] = coerce(record[i], typeOf(header[i], declaredTypes));
    }

    rows.push(row);

    const limitError = check({
      bytes,
      header,
      fields: record,
      rowNumber,
      recordCount: rows.length,
    });
    if (limitError) {
      return { header, rows, error: limitError };
    }

    record = tokenizer.nextRecord();
  }

  return { header, rows, error: null };
}

module.exports = {
  parseSheet,
};

'use strict';

/**
 * Record decoding for uploaded spreadsheet exports.
 *
 * The decoder walks the input once, character by character. Newlines are part
 * of the decoding state rather than a pre-splitting step, so quoted fields can
 * span physical lines. See docs/csv-intake.md section 2.
 */

const QUOTE = '"';

/**
 * Decode a whole input into records.
 *
 * Returns `{ records, error }`. On a structural failure `error` describes the
 * position (1-based record number and character offset) and `records` holds
 * every complete record decoded before the failure; the partial record is not
 * emitted.
 */
function decodeText(text, delimiter) {
  const records = [];
  let record = [''];
  let field = 0;
  let recordNumber = 1;
  let column = 0;
  let quoteStart = 0;
  let quoted = false;
  let afterQuote = false;

  const fail = (code, message) => ({
    code,
    message,
    row: recordNumber,
    column,
  });

  for (let i = 0; i < text.length; i += 1) {
    const ch = text.charAt(i);

    if (quoted) {
      if (ch === QUOTE) {
        if (text.charAt(i + 1) === QUOTE) {
          record[field] += QUOTE;
          i += 1;
          column += 2;
        } else {
          quoted = false;
          afterQuote = true;
          column += 1;
        }
      } else {
        record[field] += ch;
        column += 1;
      }
      continue;
    }

    if (ch === QUOTE && record[field] === '' && !afterQuote) {
      quoted = true;
      quoteStart = column + 1;
      column += 1;
      continue;
    }

    if (afterQuote && ch !== delimiter && ch !== '\r' && ch !== '\n') {
      return {
        records,
        error: {
          code: 'fatal_unexpected_character',
          message: `record ${recordNumber}: unexpected character ${JSON.stringify(ch)} after closing quote`,
          row: recordNumber,
          column: column + 1,
        },
      };
    }

    if (ch === delimiter) {
      field += 1;
      record.push('');
      afterQuote = false;
      column += 1;
      continue;
    }

    if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text.charAt(i + 1) === '\n') {
        i += 1;
      }
      records.push(record);
      record = [''];
      field = 0;
      recordNumber += 1;
      column = 0;
      afterQuote = false;
      continue;
    }

    record[field] += ch;
    column += 1;
    afterQuote = false;
  }

  if (quoted) {
    return {
      records,
      error: Object.assign(fail(
        'fatal_unterminated_quote',
        `record ${recordNumber}: quoted field reaches end of input without a closing quote`,
      ), { column: quoteStart }),
    };
  }

  const hadContent = column > 0;
  if (hadContent) {
    records.push(record);
  } else if (
    records.length > 0 &&
    (text.endsWith('\n') || text.endsWith('\r'))
  ) {
    const last = records[records.length - 1];
    if (last.length === 1 && last[0] === '') {
      records.pop();
    }
  }

  return { records, error: null };
}

/**
 * Split one logical record (without a line terminator) into fields.
 */
function splitFields(line, delimiter) {
  return decodeText(line, delimiter).records[0] || [''];
}

/**
 * @param {string} text
 * @param {string} delimiter
 */
function createTokenizer(text, delimiter = ',') {
  const decoded = decodeText(text, delimiter);
  const records = decoded.records;
  let cursor = 0;
  let recordNumber = 0;

  return {
    get recordNumber() {
      return recordNumber;
    },

    get exhausted() {
      return cursor >= records.length;
    },

    /** @type {null | { code: string, message: string, row: number, column: number }} */
    get error() {
      return decoded.error;
    },

    /**
     * @returns {string[] | null}
     */
    nextRecord() {
      if (cursor >= records.length) {
        return null;
      }

      recordNumber += 1;
      return records[cursor++];
    },
  };
}

module.exports = {
  createTokenizer,
  decodeText,
  splitFields,
};

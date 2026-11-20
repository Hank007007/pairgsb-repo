'use strict';

/**
 * Record decoding for uploaded spreadsheet exports.
 *
 * A tokenizer is created for one input and handed a delimiter. Calling
 * `nextRecord()` returns the next record as an array of decoded field strings,
 * or `null` once the input is exhausted.
 */

const QUOTE = '"';
const BACKSLASH = '\\';

/**
 * Read a field that starts with a quote character.
 *
 * Returns the decoded value and the index just past the closing quote. When no
 * closing quote is found the remaining text is taken as the value.
 */
function readQuotedField(line, start) {
  let value = '';
  let i = start + 1;

  while (i < line.length) {
    const ch = line.charAt(i);

    if (ch === BACKSLASH && line.charAt(i + 1) === QUOTE) {
      value += QUOTE;
      i += 2;
      continue;
    }

    if (ch === QUOTE) {
      return { value, next: i + 1 };
    }

    value += ch;
    i += 1;
  }

  return { value, next: i };
}

/**
 * Split one physical line into fields.
 */
function splitFields(line, delimiter) {
  const fields = [];
  let i = 0;

  while (true) {
    if (i < line.length && line.charAt(i) === QUOTE) {
      const quoted = readQuotedField(line, i);
      let field = quoted.value;

      if (quoted.next < line.length && line.charAt(quoted.next) !== delimiter) {
        // Anything that follows the closing quote on the line stays with the
        // field.
        field += line.slice(quoted.next);
        i = line.length;
      } else {
        i = quoted.next;
      }

      fields.push(field);
    } else {
      let end = i;
      while (end < line.length && line.charAt(end) !== delimiter) {
        end += 1;
      }
      fields.push(line.slice(i, end));
      i = end;
    }

    if (i >= line.length) {
      break;
    }

    // Skip the delimiter. A delimiter at end of line produces a final empty
    // field.
    i += 1;
    if (i >= line.length) {
      fields.push('');
      break;
    }
  }

  return fields;
}

/**
 * Split the input into physical lines. A trailing terminator does not produce
 * an extra empty line.
 */
function physicalLines(text) {
  const lines = text.split('\n');
  if (lines.length > 0 && lines[lines.length - 1] === '') {
    lines.pop();
  }
  return lines;
}

/**
 * @param {string} text
 * @param {string} delimiter
 */
function createTokenizer(text, delimiter = ',') {
  const lines = physicalLines(text);
  let cursor = 0;
  let recordNumber = 0;

  return {
    /** @type {null | { code: string, message: string, row: number, column: number }} */
    error: null,

    get recordNumber() {
      return recordNumber;
    },

    get exhausted() {
      return cursor >= lines.length;
    },

    /**
     * @returns {string[] | null}
     */
    nextRecord() {
      if (cursor >= lines.length) {
        return null;
      }

      let line = lines[cursor];
      cursor += 1;
      recordNumber += 1;

      if (line.endsWith('\r')) {
        line = line.slice(0, -1);
      }

      return splitFields(line, delimiter);
    },
  };
}

module.exports = {
  createTokenizer,
  splitFields,
  readQuotedField,
};

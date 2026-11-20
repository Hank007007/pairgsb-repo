'use strict';

/**
 * Cell coercion and export formatting.
 *
 * Intake turns raw field text into typed values with `coerce`. The export path
 * turns a typed value back into a field a spreadsheet can open safely with
 * `exportCell`.
 */

const ISO_DATE =
  /^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}):(\d{2}))?$/;

/**
 * Coerce one decoded field to the declared column type.
 *
 * @param {string} raw
 * @param {'text' | 'number' | 'date'} [type]
 */
function coerce(raw, type = 'text') {
  if (type === 'number') {
    if (raw === '') {
      return null;
    }
    return parseFloat(raw);
  }

  if (type === 'date') {
    if (raw === '') {
      return null;
    }

    const match = ISO_DATE.exec(raw);
    if (!match) {
      throw new TypeError(`cannot read ${JSON.stringify(raw)} as a date`);
    }

    const [, year, month, day, hour = '00', minute = '00', second = '00'] = match;
    const value = new Date(
      Date.UTC(
        Number(year),
        Number(month) - 1,
        Number(day),
        Number(hour),
        Number(minute),
        Number(second),
      ),
    );

    return value;
  }

  return raw;
}

/**
 * Neutralise a value that a spreadsheet would otherwise evaluate as a formula.
 *
 * @param {unknown} value
 * @returns {string}
 */
function sanitizeCell(value) {
  const text = value === null || value === undefined ? '' : String(value);

  if (text.charAt(0) === '=') {
    return "'" + text;
  }

  return text;
}

/**
 * Render a typed value as plain text for export.
 */
function renderScalar(value) {
  if (value === null || value === undefined) {
    return '';
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  return String(value);
}

/**
 * Wrap a field in quotes when it contains a delimiter, quote or newline.
 */
function quoteCell(text) {
  if (text === '') {
    return '';
  }
  if (/[",\n\r]/.test(text)) {
    return '"' + text + '"';
  }
  return text;
}

/**
 * Format one cell for a spreadsheet export.
 *
 * @param {unknown} value
 * @param {'text' | 'number' | 'date'} [type]
 */
function exportCell(value, type = 'text') {
  if (type === 'number' || type === 'date') {
    return quoteCell(renderScalar(value));
  }

  return quoteCell(sanitizeCell(value));
}

module.exports = {
  coerce,
  exportCell,
  quoteCell,
  renderScalar,
  sanitizeCell,
};

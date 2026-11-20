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

const PLAIN_NUMBER = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/;

/**
 * A plain decimal number per §5.3: optional sign, digits, optional decimal
 * point. Exponents, separators, NaN and Infinity are deliberately excluded.
 */
function isGenuineNumber(text) {
  return PLAIN_NUMBER.test(text);
}

/**
 * An ISO date or date-time per §5.5.
 */
function isGenuineDate(text) {
  return ISO_DATE.test(text);
}

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
    if (!isGenuineNumber(raw)) {
      const error = new TypeError(
        `cannot read ${JSON.stringify(raw)} as a number`,
      );
      error.code = 'fatal_invalid_number';
      throw error;
    }
    return Number(raw);
  }

  if (type === 'date') {
    if (raw === '') {
      return null;
    }

    const match = ISO_DATE.exec(raw);
    if (!match) {
      const error = new TypeError(
        `cannot read ${JSON.stringify(raw)} as a date`,
      );
      error.code = 'fatal_invalid_date';
      throw error;
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

  if (text === '') {
    return '';
  }

  if (isGenuineNumber(text) || isGenuineDate(text)) {
    return text;
  }

  const stripped = text.replace(/^[ \n\f\v\0]+/, '');
  const first = stripped.charAt(0);
  if (first === '=' || first === '+' || first === '-' ||
      first === '@' || first === '\t' || first === '\r') {
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
    return '"' + text.replace(/"/g, '""') + '"';
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
  return quoteCell(sanitizeCell(renderScalar(value)));
}

module.exports = {
  coerce,
  exportCell,
  isGenuineDate,
  isGenuineNumber,
  quoteCell,
  renderScalar,
  sanitizeCell,
};

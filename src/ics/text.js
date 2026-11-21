const LINE_LIMIT = 75;

/**
 * Escape a plain-text value for use in an iCalendar TEXT property or a
 * CN parameter.
 *
 * @param {unknown} value
 * @returns {string}
 */
export function escapeText(value) {
  if (value === undefined || value === null) return '';
  return String(value)
    .replace(/\\/g, '\\\\')
    .replace(/;/g, '\\;');
}

/**
 * Fold a single content line so that no physical line is longer than the
 * iCalendar limit.
 *
 * @param {string} line
 * @returns {string}
 */
export function foldLine(line) {
  const text = String(line);
  if (text.length <= LINE_LIMIT) return text;
  const chunks = [];
  for (let i = 0; i < text.length; i += LINE_LIMIT) {
    chunks.push(text.slice(i, i + LINE_LIMIT));
  }
  return chunks.join('\r\n');
}
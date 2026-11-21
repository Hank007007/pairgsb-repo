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
    .replace(/;/g, '\\;')
    .replace(/,/g, '\\,')
    .replace(/\r\n|\r|\n/g, '\\n');
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
  if (Buffer.byteLength(text, 'utf8') <= LINE_LIMIT) return text;

  const chars = Array.from(text);
  const tokens = [];
  for (let i = 0; i < chars.length; i += 1) {
    const ch = chars[i];
    if (
      ch === '\\' &&
      i + 1 < chars.length &&
      (chars[i + 1] === '\\' ||
        chars[i + 1] === ';' ||
        chars[i + 1] === ',' ||
        chars[i + 1] === 'n')
    ) {
      tokens.push(ch + chars[i + 1]);
      i += 1;
    } else {
      tokens.push(ch);
    }
  }

  const physical = [];
  let current = '';
  let currentBytes = 0;
  let capacity = LINE_LIMIT;
  for (const token of tokens) {
    const size = Buffer.byteLength(token, 'utf8');
    if (currentBytes + size > capacity) {
      physical.push(current);
      current = '';
      currentBytes = 0;
      capacity = LINE_LIMIT - 1;
    }
    current += token;
    currentBytes += size;
  }
  physical.push(current);
  return physical.join('\r\n ');
}

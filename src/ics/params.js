const TOKEN_ONLY = /^[A-Za-z0-9\-+.]+$/;
const RAW_LINE_BREAK = /[\r\n]/;

/**
 * Render a single property parameter value.
 *
 * @param {unknown} value
 * @returns {string}
 */
export function quoteParam(value) {
  if (value === undefined || value === null) return '';
  if (typeof value !== 'string') {
    throw new TypeError('parameter value must be a string');
  }
  if (RAW_LINE_BREAK.test(value)) {
    throw new TypeError('parameter value must not contain a raw line break');
  }
  if (value.length === 0) return '""';
  if (TOKEN_ONLY.test(value)) return value;
  const escaped = value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  return '"' + escaped + '"';
}

/**
 * Render a CN parameter whose value has already been escaped as TEXT.
 *
 * The backslashes introduced by TEXT escaping are left untouched so the
 * value is never double-escaped; only the surrounding parameter quotes and
 * the double quotes they require are added.
 *
 * @param {string} escapedValue
 * @returns {string}
 */
export function quoteCn(escapedValue) {
  const text = String(escapedValue ?? '');
  if (text.length === 0) return '""';
  if (TOKEN_ONLY.test(text)) return text;
  return '"' + text.replace(/"/g, '\\"') + '"';
}

/**
 * Render a group of parameters, without the leading separator.
 *
 * @param {Record<string, unknown>|Map<string, unknown>} params
 * @returns {string}
 */
export function paramList(params) {
  if (!params) return '';
  const entries = params instanceof Map ? [...params] : Object.entries(params);
  const rendered = [];
  for (const [name, value] of entries) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) {
      if (value.length === 0) continue;
      for (const item of value) {
        rendered.push(name + '=' + quoteParam(item));
      }
    } else if (name === 'CN') {
      rendered.push(name + '=' + quoteCn(value));
    } else {
      rendered.push(name + '=' + quoteParam(value));
    }
  }
  return rendered.join(';');
}

const NEEDS_QUOTES = /\s/;

/**
 * Render a single property parameter value.
 *
 * @param {unknown} value
 * @returns {string}
 */
export function quoteParam(value) {
  if (value === undefined || value === null) return '';
  const text = String(value);
  if (text.length === 0) return '';
  if (!NEEDS_QUOTES.test(text)) return text;
  return '"' + text + '"';
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
      rendered.push(name + '=' + quoteParam(value.join(',')));
    } else {
      rendered.push(name + '=' + quoteParam(value));
    }
  }
  return rendered.join(';');
}
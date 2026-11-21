const ACCEPTED = new Set([
  'FREQ',
  'INTERVAL',
  'COUNT',
  'UNTIL',
  'BYDAY',
  'BYMONTHDAY',
  'BYMONTH',
  'WKST',
]);

/**
 * Parse an RRULE value into a plain descriptor.
 *
 * @param {string} rule
 * @returns {Record<string, string>|null}
 */
export function parseRRule(rule) {
  if (rule === undefined || rule === null) return null;
  const text = String(rule).trim();
  if (text.length === 0) return null;
  const out = {};
  for (const part of text.split(';')) {
    if (part.length === 0) continue;
    const eq = part.indexOf('=');
    if (eq < 0) continue;
    const name = part.slice(0, eq).trim().toUpperCase();
    const value = part.slice(eq + 1).trim();
    if (!ACCEPTED.has(name)) continue;
    if (name === 'COUNT' && 'UNTIL' in out) delete out.UNTIL;
    if (name === 'UNTIL' && 'COUNT' in out) delete out.COUNT;
    out[name] = value;
  }
  return out;
}

/**
 * Materialise the occurrences of a rule inside a window.
 *
 * @param {Record<string, string>|string|null} rule
 * @param {{ start: Date, end: Date }} window
 * @param {string} [timeZone]
 * @returns {Date[]}
 */
export function expandOccurrences(rule, window, timeZone) {
  const parsed = typeof rule === 'string' ? parseRRule(rule) : rule;
  if (!parsed || !window || !window.start || !window.end) return [];
  const start = new Date(window.start);
  const end = new Date(window.end);
  const count = Number.parseInt(parsed.COUNT, 10);
  const steps = Number.isFinite(count) && count > 0 ? count : 0;
  const interval = Math.max(1, Number.parseInt(parsed.INTERVAL, 10) || 1);

  const out = [];
  for (let i = 0; i < steps; i++) {
    const at = new Date(start.getTime());
    switch (parsed.FREQ) {
      case 'DAILY':
        at.setUTCDate(at.getUTCDate() + i * interval);
        break;
      case 'WEEKLY':
        at.setUTCDate(at.getUTCDate() + i * interval * 7);
        break;
      case 'MONTHLY':
        at.setUTCMonth(at.getUTCMonth() + i * interval);
        break;
      case 'YEARLY':
        at.setUTCFullYear(at.getUTCFullYear() + i * interval);
        break;
      case 'HOURLY':
        at.setUTCHours(at.getUTCHours() + i * interval);
        break;
      case 'MINUTELY':
        at.setUTCMinutes(at.getUTCMinutes() + i * interval);
        break;
      default:
        return out;
    }
    if (parsed.UNTIL && at.toISOString().slice(0, 19) > String(parsed.UNTIL).slice(0, 19)) {
      break;
    }
    if (at >= end) break;
    if (at >= start) out.push(at);
  }
  return out;
}
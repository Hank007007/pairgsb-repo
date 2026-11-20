const DEFAULT_TTL_MS = 15 * 60 * 1000;
const MAX_TTL_MS = 7 * 24 * 60 * 60 * 1000;

export const DEFAULT_PURPOSES = Object.freeze([
  'password-reset',
  'invite',
  'api-key-handoff',
]);

export { DEFAULT_TTL_MS, MAX_TTL_MS };

export function toMillis(value) {
  if (value instanceof Date) {
    return value.getTime();
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === 'string') {
    const parsed = Date.parse(value);
    if (!Number.isNaN(parsed)) {
      return parsed;
    }
  }
  throw new TypeError('expected a Date, timestamp or ISO-8601 string');
}

export function toDate(value) {
  return new Date(toMillis(value));
}

export function addMilliseconds(value, amount) {
  return new Date(toMillis(value) + amount);
}

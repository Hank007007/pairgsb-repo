export const DEFAULT_LIMIT = 50;
export const DEFAULT_OFFSET = 0;
export const MAX_LIMIT = 1000;
export const MAX_OFFSET = 1000000;

/**
 * Append a LIMIT/OFFSET clause to a SELECT statement.
 *
 * A missing limit or offset falls back to the defaults.
 */
export function applyPage(sql, options = {}) {
  const rawLimit = options.limit;
  const rawOffset = options.offset;
  const limit = rawLimit === undefined || rawLimit === null ? DEFAULT_LIMIT : rawLimit;
  const offset = rawOffset === undefined || rawOffset === null ? DEFAULT_OFFSET : rawOffset;
  return `${sql} LIMIT ${limit} OFFSET ${offset}`;
}

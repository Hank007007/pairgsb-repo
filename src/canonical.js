/**
 * 记录规范化：把一条审计记录变成一串确定的字节，供 HMAC 使用。
 */

const FIELD_ORDER = [
  'id',
  'ts',
  'actor',
  'action',
  'target',
  'result',
  'meta',
  'seq',
  'segment',
];

export function canonicalize(record) {
  const parts = [];
  for (const key of FIELD_ORDER) {
    if (!(key in record)) continue;
    parts.push(`${key}=${encodeValue(record[key])}`);
  }
  return parts.join('\n');
}

function encodeValue(value) {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  return JSON.stringify(value);
}

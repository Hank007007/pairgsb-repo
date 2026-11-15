import { createHmac, timingSafeEqual } from 'node:crypto';

import { canonicalize } from './canonical.js';

const KEY_ENV = 'CHAINLOG_KEY';

export function getKey() {
  const raw = process.env[KEY_ENV];
  if (!raw) {
    throw new Error(`缺少环境变量 ${KEY_ENV}`);
  }
  return Buffer.from(raw, 'utf8');
}

export function recordMac(record, key = getKey()) {
  return createHmac('sha256', key).update(canonicalize(record), 'utf8').digest('hex');
}

export function macEquals(a, b) {
  if (typeof a !== 'string' || typeof b !== 'string') return false;
  const left = Buffer.from(a, 'hex');
  const right = Buffer.from(b, 'hex');
  if (left.length !== right.length) return false;
  return timingSafeEqual(left, right);
}

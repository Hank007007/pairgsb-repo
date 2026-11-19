import { deepClone } from './util.js';

function isMergeable(value) {
  return value !== null && (typeof value === 'object' || typeof value === 'function');
}

/**
 * Recursively merge `source` into `target` and return `target`.
 *
 * Plain objects are merged key by key, arrays replace the value they find,
 * and every other value is copied in.
 */
export function deepMerge(target, source) {
  for (const key in source) {
    const incoming = source[key];
    if (Array.isArray(incoming)) {
      target[key] = deepClone(incoming);
      continue;
    }
    if (isMergeable(incoming)) {
      if (!isMergeable(target[key]) || Array.isArray(target[key])) {
        target[key] = {};
      }
      deepMerge(target[key], incoming);
      continue;
    }
    target[key] = deepClone(incoming);
  }
  return target;
}

import {
  deepClone,
  defineValue,
  isPlainObject,
  sanitizeJson,
} from './util.js';

const hasOwn = (target, key) => Object.prototype.hasOwnProperty.call(target, key);

function mergeClean(target, source) {
  for (const key of Object.keys(source)) {
    const incoming = source[key];
    if (Array.isArray(incoming)) {
      defineValue(target, key, deepClone(incoming));
      continue;
    }
    if (isPlainObject(incoming)) {
      const current = hasOwn(target, key) ? target[key] : undefined;
      if (!isPlainObject(current)) {
        defineValue(target, key, {});
      }
      mergeClean(target[key], incoming);
      continue;
    }
    defineValue(target, key, deepClone(incoming));
  }
  return target;
}

/**
 * Recursively merge `source` into `target` and return `target`.
 *
 * Plain objects are merged key by key, arrays replace the value they find,
 * and every other value is copied in. Keys and values that cannot enter the
 * document are dropped; when `options.errors` is supplied they are reported
 * through it.
 */
export function deepMerge(target, source, options = {}) {
  const { errors, path = '', style = 'dotted' } = options;
  const clean = sanitizeJson(source, { errors, path, style });
  if (!clean.ok || !isPlainObject(clean.value)) return target;
  return mergeClean(target, clean.value);
}

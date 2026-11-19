/**
 * Dotted path syntax shared by `--set` and by the environment overlay.
 */

import { defineValue, isPlainObject } from './util.js';

const hasOwn = (target, key) => Object.prototype.hasOwnProperty.call(target, key);

export function splitDottedPath(pathText) {
  return String(pathText).split('.');
}

/**
 * Read the value stored at `segments` in `root`.
 * Returns `{ found, value }`; `found` is false when any segment is absent.
 */
export function readPath(root, segments) {
  let cursor = root;
  for (const segment of segments) {
    if (cursor === null || (typeof cursor !== 'object' && typeof cursor !== 'function')) {
      return { found: false };
    }
    if (!Object.prototype.hasOwnProperty.call(cursor, segment)) {
      return { found: false };
    }
    cursor = cursor[segment];
  }
  return { found: true, value: cursor };
}

/**
 * Write `value` at the dotted path `pathText`, creating intermediate
 * containers as needed.
 */
export function setPath(target, pathText, value) {
  const segments = splitDottedPath(pathText);
  if (segments.length === 0) return target;
  let cursor = target;
  for (let index = 0; index < segments.length - 1; index += 1) {
    const key = segments[index];
    const next = hasOwn(cursor, key) ? cursor[key] : undefined;
    if (!isPlainObject(next) && !Array.isArray(next)) {
      defineValue(cursor, key, {});
    }
    cursor = cursor[key];
  }
  const last = segments[segments.length - 1];
  defineValue(cursor, last, value);
  return target;
}

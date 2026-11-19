/**
 * Small structural helpers shared by the merge, patch and loader modules.
 */

import { CODES, makeError } from './errors.js';

export const RESERVED_KEYS = new Set([
  '__proto__',
  'constructor',
  'prototype',
  'length',
]);

const CONTROL_CHARACTERS = /[\u0000-\u001f\u007f]/;

export function isPlainObject(value) {
  if (value === null || typeof value !== 'object') return false;
  const proto = Object.getPrototypeOf(value);
  return proto === Object.prototype || proto === null;
}

/**
 * Classify a key against the document whitelist (merge-rules §2/§3).
 * Path-separator checks are surface-specific and happen before this call.
 */
export function classifyKey(key) {
  if (typeof key !== 'string' || key.length === 0) {
    return { ok: false, code: CODES.INVALID_KEY };
  }
  if (RESERVED_KEYS.has(key)) {
    return { ok: false, code: CODES.RESERVED_KEY };
  }
  if (CONTROL_CHARACTERS.test(key)) {
    return { ok: false, code: CODES.INVALID_KEY };
  }
  return { ok: true };
}

export function defineValue(target, key, value) {
  Object.defineProperty(target, key, {
    value,
    enumerable: true,
    writable: true,
    configurable: true,
  });
  return target;
}

export function deepClone(value) {
  if (Array.isArray(value)) {
    return value.map((item) => deepClone(item));
  }
  if (value !== null && typeof value === 'object') {
    const copy = {};
    for (const key of Object.keys(value)) {
      defineValue(copy, key, deepClone(value[key]));
    }
    return copy;
  }
  return value;
}

export function deepEqual(left, right) {
  if (left === right) return true;
  if (typeof left !== typeof right) return false;
  if (left === null || right === null) return false;
  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right)) return false;
    if (left.length !== right.length) return false;
    return left.every((item, index) => deepEqual(item, right[index]));
  }
  if (typeof left === 'object') {
    const leftKeys = Object.keys(left);
    const rightKeys = Object.keys(right);
    if (leftKeys.length !== rightKeys.length) return false;
    return leftKeys.every(
      (key) =>
        Object.prototype.hasOwnProperty.call(right, key) &&
        deepEqual(left[key], right[key]),
    );
  }
  return false;
}

function childPath(base, key, style) {
  if (style === 'pointer') {
    const encoded = String(key).replace(/~/g, '~0').replace(/\//g, '~1');
    return `${base}/${encoded}`;
  }
  return base ? `${base}.${key}` : String(key);
}

/**
 * Clone a value into the document's JSON-only shape, dropping (and reporting)
 * keys or values that are not allowed to enter the document.
 *
 * @param {*} value
 * @param {{
 *   errors?: Array<{code: string, path: string, message: string}>,
 *   path?: string,
 *   style?: 'dotted' | 'pointer',
 *   seen?: WeakSet<object>,
 * }} options
 * @returns {{ ok: boolean, value?: * }}
 */
export function sanitizeJson(value, options = {}) {
  const {
    errors,
    path = '',
    style = 'dotted',
    seen = new WeakSet(),
  } = options;

  const report = (code, at, message) => {
    if (errors) errors.push(makeError(code, at, message));
  };
  const fail = (message) => {
    report(CODES.TYPE, path, message);
    return { ok: false };
  };

  if (value === null) return { ok: true, value: null };

  const type = typeof value;
  if (type === 'string' || type === 'boolean') {
    return { ok: true, value };
  }
  if (type === 'number') {
    if (!Number.isFinite(value)) return fail('number is not a JSON value');
    return { ok: true, value };
  }
  if (type !== 'object') {
    return fail(`value of type ${type} is not a JSON value`);
  }
  if (seen.has(value)) {
    return fail('cyclic value is not a JSON value');
  }
  seen.add(value);

  if (Array.isArray(value)) {
    const copy = [];
    for (let index = 0; index < value.length; index += 1) {
      const itemPath = style === 'pointer'
        ? `${path}/${index}`
        : childPath(path, index, style);
      const item = sanitizeJson(value[index], {
        errors,
        path: itemPath,
        style,
        seen,
      });
      if (item.ok) copy.push(item.value);
    }
    seen.delete(value);
    return { ok: true, value: copy };
  }

  if (!isPlainObject(value)) {
    seen.delete(value);
    return fail('value is not a plain object');
  }

  const copy = {};
  for (const key of Reflect.ownKeys(value)) {
    if (typeof key !== 'string') {
      report(CODES.INVALID_KEY, path, 'symbol keys are not allowed');
      continue;
    }
    const at = childPath(path, key, style);
    const classification = classifyKey(key);
    if (!classification.ok) {
      report(classification.code, at, `key ${JSON.stringify(key)} is not allowed`);
      continue;
    }
    let raw;
    try {
      raw = value[key];
    } catch {
      report(CODES.TYPE, at, 'property could not be read');
      continue;
    }
    const item = sanitizeJson(raw, { errors, path: at, style, seen });
    if (item.ok) defineValue(copy, key, item.value);
  }
  seen.delete(value);
  return { ok: true, value: copy };
}

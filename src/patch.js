import { CODES, makeError } from './errors.js';
import {
  deepClone,
  deepEqual,
  defineValue,
  isPlainObject,
  RESERVED_KEYS,
  sanitizeJson,
} from './util.js';

const SUPPORTED_OPERATIONS = new Set([
  'add',
  'remove',
  'replace',
  'move',
  'copy',
  'test',
]);

const CONTROL_CHARACTERS = /[\u0000-\u001f\u007f]/;
const CANONICAL_INDEX = /^(0|[1-9][0-9]*)$/;
const hasOwn = (target, key) => Object.prototype.hasOwnProperty.call(target, key);

/**
 * Decode one JSON Pointer segment (`~1` is `/`, `~0` is `~`).
 */
function decodeSegment(segment) {
  return segment.replace(/~1/g, '/').replace(/~0/g, '~');
}

function parsePointer(pointer, { allowWhole = false } = {}) {
  if (typeof pointer !== 'string') {
    return { ok: false, code: CODES.PATCH_PATH, path: String(pointer ?? '') };
  }
  if (pointer === '') {
    if (allowWhole) return { ok: true, segments: [] };
    return { ok: false, code: CODES.PATCH_PATH, path: pointer };
  }
  if (!pointer.startsWith('/')) {
    return { ok: false, code: CODES.PATCH_PATH, path: pointer };
  }

  const segments = pointer.slice(1).split('/').map(decodeSegment);
  for (const segment of segments) {
    if (segment === '.' || segment === '..') {
      return { ok: false, code: CODES.PATCH_PATH, path: pointer };
    }
    if (CONTROL_CHARACTERS.test(segment)) {
      return { ok: false, code: CODES.INVALID_KEY, path: pointer };
    }
  }
  return { ok: true, segments };
}

function canonicalIndex(segment) {
  if (!CANONICAL_INDEX.test(segment)) return null;
  const index = Number(segment);
  return Number.isSafeInteger(index) ? index : null;
}

function checkSegment(segment, container, { isFinal, op }) {
  const isArray = Array.isArray(container);

  if (segment === '') {
    return { ok: false, code: isArray ? CODES.PATCH_PATH : CODES.INVALID_KEY };
  }
  if (CONTROL_CHARACTERS.test(segment)) {
    return { ok: false, code: CODES.INVALID_KEY };
  }
  if (RESERVED_KEYS.has(segment)) {
    if (isArray && segment === 'length') {
      return { ok: false, code: CODES.PATCH_PATH };
    }
    return { ok: false, code: CODES.RESERVED_KEY };
  }
  if (isArray) {
    if (segment === '-' && isFinal && op === 'add') {
      return { ok: true, index: container.length, append: true };
    }
    const index = canonicalIndex(segment);
    if (index === null) {
      return { ok: false, code: CODES.PATCH_PATH };
    }
    return { ok: true, index };
  }
  if (segment === '-') {
    return { ok: false, code: CODES.PATCH_PATH };
  }
  return { ok: true, key: segment };
}

function reject(errors, code, path, message) {
  errors.push(makeError(code, path, message));
  return null;
}

/**
 * Walk the parent of `segments`, creating missing plain-object intermediates.
 * Returns where the final value should be inserted.
 */
function prepareAdd(document, segments, pointer, errors) {
  let cursor = document;

  for (let index = 0; index < segments.length - 1; index += 1) {
    const segment = segments[index];
    if (cursor === null || typeof cursor !== 'object') {
      return reject(errors, CODES.PATCH_PATH, pointer, 'intermediate is not a container');
    }

    if (Array.isArray(cursor)) {
      const check = checkSegment(segment, cursor, { isFinal: false, op: 'add' });
      if (!check.ok) {
        return reject(errors, check.code, pointer, 'invalid array index');
      }
      if (check.append) {
        return reject(errors, CODES.PATCH_PATH, pointer, 'invalid intermediate index');
      }
      if (check.index > cursor.length) {
        return reject(errors, CODES.PATCH_PATH, pointer, 'array index is out of range');
      }
      if (check.index === cursor.length) {
        cursor.push({});
      } else if (!hasOwn(cursor, check.index)) {
        defineValue(cursor, check.index, {});
      }
      const next = cursor[check.index];
      if (!isPlainObject(next) && !Array.isArray(next)) {
        return reject(errors, CODES.PATCH_PATH, pointer, 'intermediate is not a container');
      }
      cursor = next;
      continue;
    }

    const check = checkSegment(segment, cursor, { isFinal: false, op: 'add' });
    if (!check.ok) {
      return reject(errors, check.code, pointer, 'invalid object key');
    }
    if (!hasOwn(cursor, check.key)) {
      defineValue(cursor, check.key, {});
    }
    const next = cursor[check.key];
    if (!isPlainObject(next) && !Array.isArray(next)) {
      return reject(errors, CODES.PATCH_PATH, pointer, 'intermediate is not a container');
    }
    cursor = next;
  }

  if (cursor === null || typeof cursor !== 'object') {
    return reject(errors, CODES.PATCH_PATH, pointer, 'target parent is not a container');
  }

  const last = segments[segments.length - 1];
  const check = checkSegment(last, cursor, { isFinal: true, op: 'add' });
  if (!check.ok) {
    return reject(errors, check.code, pointer, 'invalid target key');
  }

  if (Array.isArray(cursor)) {
    const index = check.append ? cursor.length : check.index;
    if (index > cursor.length) {
      return reject(errors, CODES.PATCH_PATH, pointer, 'array index is out of range');
    }
    return { parent: cursor, kind: 'array', index };
  }
  return { parent: cursor, kind: 'object', key: check.key };
}

/**
 * Resolve an existing location, without ever consulting the prototype chain.
 */
function resolveExisting(document, segments, pointer, errors) {
  let cursor = document;

  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    const isFinal = index === segments.length - 1;

    if (cursor === null || typeof cursor !== 'object') {
      return reject(errors, CODES.PATCH_PATH, pointer, 'intermediate is not a container');
    }

    const check = checkSegment(segment, cursor, { isFinal, op: 'lookup' });
    if (!check.ok) {
      return reject(errors, check.code, pointer, 'invalid path segment');
    }

    if (Array.isArray(cursor)) {
      if (check.append || check.index >= cursor.length || !hasOwn(cursor, check.index)) {
        const code = isFinal ? CODES.PATCH_NOT_FOUND : CODES.PATCH_PATH;
        return reject(errors, code, pointer, 'path does not exist');
      }
      if (isFinal) {
        return {
          parent: cursor,
          kind: 'array',
          index: check.index,
          value: cursor[check.index],
        };
      }
      cursor = cursor[check.index];
      continue;
    }

    if (!hasOwn(cursor, check.key)) {
      const code = isFinal ? CODES.PATCH_NOT_FOUND : CODES.PATCH_PATH;
      return reject(errors, code, pointer, 'path does not exist');
    }
    if (isFinal) {
      return {
        parent: cursor,
        kind: 'object',
        key: check.key,
        value: cursor[check.key],
      };
    }
    cursor = cursor[check.key];
  }

  return reject(errors, CODES.PATCH_PATH, pointer, 'the document root is not addressable here');
}

function insertAt(target, value) {
  if (target.kind === 'array') {
    target.parent.splice(target.index, 0, value);
    return;
  }
  defineValue(target.parent, target.key, value);
}

function replaceAt(target, value) {
  if (target.kind === 'array') {
    target.parent[target.index] = value;
    return;
  }
  defineValue(target.parent, target.key, value);
}

function removeAt(target) {
  if (target.kind === 'array') {
    target.parent.splice(target.index, 1);
    return;
  }
  delete target.parent[target.key];
}

function sanitizedValue(raw, pointer, errors) {
  return sanitizeJson(raw, { errors, path: pointer, style: 'pointer' });
}

function rootReplacement(raw, pointer, errors) {
  if (!isPlainObject(raw)) {
    reject(errors, CODES.PATCH_TYPE, pointer, 'document root must be a plain object');
    return null;
  }
  const clean = sanitizeJson(raw, { errors, path: '', style: 'pointer' });
  return clean.ok ? clean.value : null;
}

function applyAdd(document, segments, raw, pointer, errors) {
  if (segments.length === 0) {
    const replacement = rootReplacement(raw, pointer, errors);
    return replacement === null ? document : replacement;
  }

  const value = sanitizedValue(raw, pointer, errors);
  if (!value.ok) return document;

  const candidate = deepClone(document);
  const target = prepareAdd(candidate, segments, pointer, errors);
  if (target === null) return document;
  insertAt(target, value.value);
  return candidate;
}

function applyRemove(document, segments, pointer, errors) {
  const candidate = deepClone(document);
  const target = resolveExisting(candidate, segments, pointer, errors);
  if (target === null) return document;
  removeAt(target);
  return candidate;
}

function applyReplace(document, segments, raw, pointer, errors) {
  if (segments.length === 0) {
    const replacement = rootReplacement(raw, pointer, errors);
    return replacement === null ? document : replacement;
  }

  const value = sanitizedValue(raw, pointer, errors);
  if (!value.ok) return document;

  const candidate = deepClone(document);
  const target = resolveExisting(candidate, segments, pointer, errors);
  if (target === null) return document;
  replaceAt(target, value.value);
  return candidate;
}

function applyTest(document, segments, expected, pointer, errors) {
  const target = resolveExisting(document, segments, pointer, errors);
  if (target === null) return document;
  if (!deepEqual(target.value, expected)) {
    errors.push(makeError(CODES.PATCH_TEST, pointer, 'value differs from expectation'));
  }
  return document;
}

function isDescendant(pathSegments, fromSegments) {
  if (pathSegments.length <= fromSegments.length) return false;
  return fromSegments.every((segment, index) => pathSegments[index] === segment);
}

function applyTransfer(document, parsedPath, rawFrom, move, errors) {
  const from = parsePointer(rawFrom, { allowWhole: false });
  if (!from.ok) {
    reject(errors, from.code, from.path, 'from must be a valid pointer');
    return document;
  }
  from.pointer = rawFrom;
  if (move && isDescendant(parsedPath.segments, from.segments)) {
    reject(errors, CODES.PATCH_PATH, parsedPath.pointer, 'path is inside from');
    return document;
  }

  const candidate = deepClone(document);
  const source = resolveExisting(candidate, from.segments, from.pointer, errors);
  if (source === null) return document;

  const value = deepClone(source.value);
  if (move) removeAt(source);

  const target = prepareAdd(candidate, parsedPath.segments, parsedPath.pointer, errors);
  if (target === null) return document;
  insertAt(target, value);
  return candidate;
}

function applyOperation(document, operation, errors) {
  const op = operation && operation.op;
  const pointer = operation && operation.path;

  if (typeof op !== 'string' || !SUPPORTED_OPERATIONS.has(op)) {
    reject(errors, CODES.PATCH_OP, typeof pointer === 'string' ? pointer : '', 'unsupported operation');
    return document;
  }
  if (typeof pointer !== 'string') {
    reject(errors, CODES.PATCH_PATH, '', 'operation has no path');
    return document;
  }

  const allowWhole = op === 'add' || op === 'replace';
  const parsed = parsePointer(pointer, { allowWhole });
  if (!parsed.ok) {
    reject(errors, parsed.code, parsed.path, 'invalid JSON Pointer');
    return document;
  }
  parsed.pointer = pointer;

  switch (op) {
    case 'add':
      return applyAdd(document, parsed.segments, operation.value, pointer, errors);
    case 'remove':
      return applyRemove(document, parsed.segments, pointer, errors);
    case 'replace':
      return applyReplace(document, parsed.segments, operation.value, pointer, errors);
    case 'move':
      return applyTransfer(document, parsed, operation.from, true, errors);
    case 'copy':
      return applyTransfer(document, parsed, operation.from, false, errors);
    case 'test':
      return applyTest(document, parsed.segments, operation.value, pointer, errors);
    default:
      return document;
  }
}

/**
 * Apply a JSON Patch style sequence to a copy of `document`.
 *
 * @param {object} document
 * @param {Array<object>} patch
 * @returns {{ document: object, errors: Array<{code: string, path: string, message: string}> }}
 */
export function applyPatch(document, patch = []) {
  const errors = [];
  let current = {};

  if (isPlainObject(document)) {
    const clean = sanitizeJson(document, { errors, path: '', style: 'pointer' });
    current = clean.ok ? clean.value : {};
  } else {
    errors.push(makeError(CODES.TYPE, '', 'document must be a plain object'));
  }

  if (!Array.isArray(patch)) {
    errors.push(makeError(CODES.PATCH_TYPE, '', 'patch must be an array of operations'));
    return { document: current, errors };
  }

  for (const operation of patch) {
    current = applyOperation(current, operation, errors);
  }

  return { document: current, errors };
}

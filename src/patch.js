import { CODES, makeError } from './errors.js';
import { deepClone, deepEqual, isPlainObject } from './util.js';

/**
 * Decode one JSON Pointer segment (`~1` is `/`, `~0` is `~`).
 */
function decodeSegment(segment) {
  return segment.replace(/~1/g, '/').replace(/~0/g, '~');
}

function segmentsOf(pointer) {
  return pointer
    .slice(1)
    .split('/')
    .map((segment) => decodeSegment(segment));
}

function isWholeDocument(pointer) {
  return pointer === '';
}

/**
 * Walk `segments` from `document`, creating ordinary containers along the way
 * when they are absent. Used by `add`, which may introduce new structure.
 */
function create(document, segments) {
  let cursor = document;
  for (const segment of segments) {
    if (cursor[segment] === null || typeof cursor[segment] !== 'object') {
      cursor[segment] = {};
    }
    cursor = cursor[segment];
  }
  return cursor;
}

/**
 * Walk `segments` from `document` without creating anything.
 */
function lookup(document, segments) {
  let cursor = document;
  for (const segment of segments) {
    if (cursor === null || typeof cursor !== 'object') {
      return { found: false };
    }
    if (!(segment in cursor)) {
      return { found: false };
    }
    cursor = cursor[segment];
  }
  return { found: true, value: cursor };
}

function arrayIndexOf(container, segment) {
  if (segment === '-') return container.length;
  return Number(segment);
}

function insertInto(container, segment, value) {
  if (Array.isArray(container)) {
    container.splice(arrayIndexOf(container, segment), 0, value);
    return;
  }
  container[segment] = value;
}

function setIn(container, segment, value) {
  if (Array.isArray(container)) {
    container[arrayIndexOf(container, segment)] = value;
    return;
  }
  container[segment] = value;
}

function removeFrom(container, segment) {
  if (Array.isArray(container)) {
    return container.splice(arrayIndexOf(container, segment), 1)[0];
  }
  const value = container[segment];
  delete container[segment];
  return value;
}

function applyAdd(document, pointer, value, errors) {
  const segments = segmentsOf(pointer);
  const parent = create(document, segments.slice(0, -1));
  insertInto(parent, segments[segments.length - 1], deepClone(value));
  return document;
}

function applyRemove(document, pointer, errors) {
  const segments = segmentsOf(pointer);
  const parent = lookup(document, segments.slice(0, -1));
  const key = segments[segments.length - 1];
  if (!parent.found || !(key in parent.value)) {
    errors.push(makeError(CODES.PATCH_NOT_FOUND, pointer, 'path does not exist'));
    return document;
  }
  removeFrom(parent.value, key);
  return document;
}

function applyReplace(document, pointer, value, errors) {
  const segments = segmentsOf(pointer);
  const parent = lookup(document, segments.slice(0, -1));
  const key = segments[segments.length - 1];
  if (!parent.found || !(key in parent.value)) {
    errors.push(makeError(CODES.PATCH_NOT_FOUND, pointer, 'path does not exist'));
    return document;
  }
  setIn(parent.value, key, deepClone(value));
  return document;
}

function applyMove(document, pointer, from, errors, removeSource) {
  if (typeof from !== 'string' || !from.startsWith('/')) {
    errors.push(makeError(CODES.PATCH_PATH, String(from), 'from must start with /'));
    return document;
  }

  const sourceSegments = segmentsOf(from);
  const parent = lookup(document, sourceSegments.slice(0, -1));
  const key = sourceSegments[sourceSegments.length - 1];
  if (!parent.found || !(key in parent.value)) {
    errors.push(makeError(CODES.PATCH_NOT_FOUND, from, 'from does not exist'));
    return document;
  }

  const value = deepClone(parent.value[key]);
  if (removeSource) removeFrom(parent.value, key);

  return applyAdd(document, pointer, value, errors);
}

function applyTest(document, pointer, expected, errors) {
  const segments = segmentsOf(pointer);
  const target = lookup(document, segments);
  if (!target.found) {
    errors.push(makeError(CODES.PATCH_NOT_FOUND, pointer, 'path does not exist'));
    return document;
  }
  if (!deepEqual(target.value, expected)) {
    errors.push(makeError(CODES.PATCH_TEST, pointer, 'value differs from expectation'));
  }
  return document;
}

function applyOperation(document, operation, errors) {
  const op = operation && operation.op;
  const pointer = operation && operation.path;

  if (typeof op !== 'string') {
    errors.push(makeError(CODES.PATCH_OP, '', 'operation has no op'));
    return document;
  }
  if (typeof pointer !== 'string') {
    errors.push(makeError(CODES.PATCH_PATH, '', 'operation has no path'));
    return document;
  }
  if (!isWholeDocument(pointer) && !pointer.startsWith('/')) {
    errors.push(makeError(CODES.PATCH_PATH, pointer, 'path must start with /'));
    return document;
  }

  switch (op) {
    case 'add':
      if (isWholeDocument(pointer)) {
        if (!isPlainObject(operation.value)) {
          errors.push(makeError(CODES.PATCH_TYPE, pointer, 'document root must be an object'));
          return document;
        }
        return deepClone(operation.value);
      }
      return applyAdd(document, pointer, operation.value, errors);
    case 'remove':
      if (isWholeDocument(pointer)) {
        errors.push(makeError(CODES.PATCH_PATH, pointer, 'the document root cannot be removed'));
        return document;
      }
      return applyRemove(document, pointer, errors);
    case 'replace':
      if (isWholeDocument(pointer)) {
        if (!isPlainObject(operation.value)) {
          errors.push(makeError(CODES.PATCH_TYPE, pointer, 'document root must be an object'));
          return document;
        }
        return deepClone(operation.value);
      }
      return applyReplace(document, pointer, operation.value, errors);
    case 'move':
      return applyMove(document, pointer, operation.from, errors, true);
    case 'copy':
      return applyMove(document, pointer, operation.from, errors, false);
    case 'test':
      return applyTest(document, pointer, operation.value, errors);
    default:
      errors.push(makeError(CODES.PATCH_OP, pointer, `unsupported operation ${op}`));
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
  let current = deepClone(document);

  if (!Array.isArray(patch)) {
    errors.push(makeError(CODES.PATCH_TYPE, '', 'patch must be an array of operations'));
    return { document: current, errors };
  }

  for (const operation of patch) {
    current = applyOperation(current, operation, errors);
  }

  return { document: current, errors };
}

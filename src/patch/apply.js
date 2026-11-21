'use strict';

const { matches, fieldValue } = require('../repo/query');
const { parse } = require('../filter/parse');
const { compileSubtree } = require('../filter/compile');
const { canWrite } = require('../auth/token');
const { badRequest, forbidden } = require('../util/errors');
const { schemaFor, sensitiveAttributeOf } = require('../scim/schema');

const RESOURCE_SCOPE = { User: 'users', Group: 'groups' };

function operationList(value) {
  if (!Array.isArray(value) || value.length === 0) {
    throw badRequest('invalidValue', 'Operations must be a non-empty array');
  }
  return value.map((entry) => {
    if (!entry || typeof entry !== 'object') {
      throw badRequest('invalidValue', 'operation must be an object');
    }
    const op = String(entry.op || '').toLowerCase();
    if (op !== 'add' && op !== 'replace' && op !== 'remove') {
      throw badRequest('invalidValue', `unsupported op ${entry.op}`);
    }
    const path = entry.path === undefined || entry.path === null
      ? null
      : String(entry.path);
    return { op, path, value: entry.value };
  });
}

function splitPath(path) {
  return String(path).split('.').filter((part) => part.length > 0);
}

function rootOf(path) {
  return String(path).split('.')[0];
}

function readElementFilter(raw) {
  const bracket = raw.indexOf('[');
  if (bracket === -1) {
    return { attr: raw, filter: null };
  }
  return {
    attr: raw.slice(0, bracket),
    filter: raw.slice(bracket + 1, raw.lastIndexOf(']')),
  };
}

function setNested(target, segments, value) {
  let cursor = target;
  for (let i = 0; i < segments.length - 1; i += 1) {
    const key = segments[i];
    const next = fieldValue(cursor, key);
    if (next && typeof next === 'object' && !Array.isArray(next)) {
      cursor = next;
    } else {
      cursor[key] = {};
      cursor = cursor[key];
    }
  }
  cursor[segments[segments.length - 1]] = value;
}

function deleteNested(target, segments) {
  let cursor = target;
  for (let i = 0; i < segments.length - 1; i += 1) {
    const next = fieldValue(cursor, segments[i]);
    if (!next || typeof next !== 'object') return false;
    cursor = next;
  }
  const key = Object.keys(cursor).find(
    (candidate) => candidate.toLowerCase() === segments[segments.length - 1].toLowerCase()
  );
  if (key === undefined) return false;
  delete cursor[key];
  return true;
}

function hasNested(target, segments) {
  let cursor = target;
  for (const segment of segments) {
    const next = fieldValue(cursor, segment);
    if (next === undefined) return { found: false, value: undefined };
    cursor = next;
  }
  return { found: true, value: cursor };
}

function appendValue(existing, value) {
  const additions = Array.isArray(value) ? value : [value];
  if (Array.isArray(existing)) return existing.concat(additions);
  return additions;
}

function applyDirect(attributes, path, operation, touched) {
  const segments = splitPath(path);
  touched.add(rootOf(path));

  if (operation.op === 'remove') {
    if (!deleteNested(attributes, segments)) {
      throw badRequest('invalidPath', `path ${path} does not exist`);
    }
    return;
  }

  const located = hasNested(attributes, segments);
  if (located.found) {
    const parentSegments = segments.slice(0, -1);
    const leaf = segments[segments.length - 1];
    const parent = parentSegments.length === 0
      ? attributes
      : hasNested(attributes, parentSegments).value;
    if (operation.op === 'add' && Array.isArray(located.value)) {
      setNested(attributes, segments, appendValue(located.value, operation.value));
      return;
    }
    if (operation.op === 'add'
      && operation.value !== undefined
      && typeof operation.value === 'object'
      && !Array.isArray(operation.value)) {
      setNested(attributes, segments, { ...located.value, ...operation.value });
      return;
    }
    if (parent && typeof parent === 'object') {
      const key = Object.keys(parent).find(
        (candidate) => candidate.toLowerCase() === leaf.toLowerCase()
      );
      const targetKey = key === undefined ? leaf : key;
      if (operation.op === 'add' && Array.isArray(located.value)) {
        parent[targetKey] = appendValue(located.value, operation.value);
      } else {
        parent[targetKey] = operation.value;
      }
      return;
    }
    setNested(attributes, segments, operation.value);
    return;
  }

  setNested(attributes, segments, operation.value);
}

function applyFiltered(attributes, path, operation, touched) {
  const { attr, filter } = readElementFilter(path);
  touched.add(path);
  const list = fieldValue(attributes, attr);
  if (!Array.isArray(list)) {
    throw badRequest('invalidPath', `path ${path} does not reference a multi-valued attribute`);
  }
  const condition = compileSubtree(parse(filter));
  const matched = list.filter((item) => matches(item, condition));
  if (matched.length === 0) {
    throw badRequest('invalidPath', `path ${path} matched no element`);
  }

  if (operation.op === 'remove') {
    for (const item of matched) {
      const index = list.indexOf(item);
      if (index !== -1) list.splice(index, 1);
    }
    return;
  }

  for (const item of matched) {
    if (Array.isArray(operation.value)) {
      const replacement = operation.value[0];
      if (replacement && typeof replacement === 'object') {
        Object.assign(item, replacement);
      } else {
        item.value = replacement;
      }
      continue;
    }
    if (operation.value !== null
      && typeof operation.value === 'object') {
      Object.assign(item, operation.value);
    } else {
      item.value = operation.value;
    }
  }
}

function applyOperation(attributes, resourceType, operation, touched) {
  if (operation.path === null) {
    if (operation.op === 'remove') {
      throw badRequest('invalidPath', 'remove requires a path');
    }
    if (operation.value === null
      || typeof operation.value !== 'object'
      || Array.isArray(operation.value)) {
      throw badRequest('invalidValue', 'operation value must be an object');
    }
    if (!schemaFor(resourceType)) {
      throw badRequest('invalidValue', `unknown resource type ${resourceType}`);
    }
    for (const [key, value] of Object.entries(operation.value)) {
      applyDirect(attributes, key, { op: operation.op, value }, touched);
    }
    return;
  }

  const parsed = readElementFilter(operation.path);
  if (parsed.filter !== null) {
    applyFiltered(attributes, operation.path, operation, touched);
    return;
  }
  applyDirect(attributes, operation.path, operation, touched);
}

function assertWritable(actor, resourceType, attribute) {
  const sensitive = sensitiveAttributeOf(resourceType, attribute);
  const required = sensitive || 'profile';
  if (!canWrite(actor.scopes, RESOURCE_SCOPE[resourceType], required)) {
    throw forbidden('insufficientScope', `missing write access to ${attribute}`);
  }
}

function applyPatch({ record, resourceType, operations, actor }) {
  if (!record || typeof record !== 'object' || !record.attributes) {
    throw badRequest('invalidValue', 'missing target resource');
  }
  const parsed = operationList(operations);
  const touched = new Set();
  for (const operation of parsed) {
    applyOperation(record.attributes, resourceType, operation, touched);
  }
  for (const attribute of touched) {
    assertWritable(actor, resourceType, attribute);
  }
  return { deleted: false };
}

module.exports = { applyPatch, operationList, readElementFilter };

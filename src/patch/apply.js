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

function clone(value) {
  return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function splitPath(path) {
  return String(path).split('.').filter((part) => part.length > 0);
}

function readElementFilter(raw) {
  const text = String(raw);
  const bracket = text.indexOf('[');
  if (bracket === -1) {
    return { attr: text, filter: null };
  }
  return {
    attr: text.slice(0, bracket),
    filter: text.slice(bracket + 1, text.lastIndexOf(']')),
  };
}

function findKey(object, name) {
  if (!isPlainObject(object)) return undefined;
  if (Object.prototype.hasOwnProperty.call(object, name)) return name;
  const wanted = String(name).toLowerCase();
  return Object.keys(object).find((key) => key.toLowerCase() === wanted);
}

function topLevelDefinition(resourceType, rawName) {
  const schema = schemaFor(resourceType);
  const wanted = String(rawName).toLowerCase();
  const key = Object.keys(schema).find((candidate) => candidate.toLowerCase() === wanted);
  if (!key) return null;
  return { key, definition: schema[key] };
}

function resolveDirectPath(resourceType, rawPath) {
  const segments = splitPath(rawPath);
  if (segments.length === 0) {
    throw badRequest('invalidPath', 'empty patch path');
  }
  const top = topLevelDefinition(resourceType, segments[0]);
  if (!top) {
    throw badRequest('invalidPath', `unknown attribute ${segments[0]}`);
  }

  const canonical = [top.key];
  let definition = top.definition;
  for (let i = 1; i < segments.length; i += 1) {
    if (!definition || definition.type !== 'complex' || !definition.sub) {
      throw badRequest('invalidPath', `unknown sub-attribute ${rawPath}`);
    }
    const wanted = segments[i].toLowerCase();
    const key = Object.keys(definition.sub).find(
      (candidate) => candidate.toLowerCase() === wanted
    );
    if (!key) {
      throw badRequest('invalidPath', `unknown sub-attribute ${rawPath}`);
    }
    canonical.push(key);
    definition = { type: definition.sub[key] };
  }
  return { root: canonical[0], segments: canonical, definition };
}

function resolveFilteredPath(resourceType, rawPath) {
  const text = String(rawPath);
  const bracket = text.indexOf('[');
  const close = text.lastIndexOf(']');
  if (bracket === -1 || close < bracket || text.slice(close + 1).trim() !== '') {
    throw badRequest('invalidPath', `malformed path ${rawPath}`);
  }
  const attr = text.slice(0, bracket).trim();
  const filter = text.slice(bracket + 1, close).trim();
  if (filter.length === 0) {
    throw badRequest('invalidPath', `empty value filter in ${rawPath}`);
  }
  const top = topLevelDefinition(resourceType, attr);
  if (!top || top.definition.type !== 'multi') {
    throw badRequest('invalidPath', `${attr} is not a multi-valued attribute`);
  }
  return { root: top.key, definition: top.definition, filter };
}

function assertObjectKeys(value, allowed, label) {
  if (!isPlainObject(value)) return;
  const known = new Set(allowed.map((key) => key.toLowerCase()));
  for (const key of Object.keys(value)) {
    if (!known.has(key.toLowerCase())) {
      throw badRequest('invalidPath', `unknown attribute ${label}.${key}`);
    }
  }
}

function validateValue(definition, value, label) {
  if (definition && definition.type === 'complex') {
    assertObjectKeys(value, Object.keys(definition.sub || {}), label);
    return;
  }
  if (definition && definition.type === 'multi') {
    const element = definition.element || {};
    const items = Array.isArray(value) ? value : [value];
    for (const item of items) {
      assertObjectKeys(item, Object.keys(element), label);
    }
  }
}

function locate(target, segments) {
  let cursor = target;
  for (const segment of segments) {
    if (!isPlainObject(cursor)) {
      return { found: false, value: undefined };
    }
    const key = findKey(cursor, segment);
    if (key === undefined) {
      return { found: false, value: undefined };
    }
    cursor = cursor[key];
  }
  return { found: true, value: cursor };
}

function existingParent(target, segments) {
  let cursor = target;
  for (let i = 0; i < segments.length - 1; i += 1) {
    const key = findKey(cursor, segments[i]);
    if (key === undefined || !isPlainObject(cursor[key])) {
      throw badRequest('invalidPath', `path ${segments.join('.')} does not exist`);
    }
    cursor = cursor[key];
  }
  return cursor;
}

function ensureParent(target, segments) {
  let cursor = target;
  for (let i = 0; i < segments.length - 1; i += 1) {
    const key = findKey(cursor, segments[i]) || segments[i];
    if (cursor[key] === undefined) {
      cursor[key] = {};
    } else if (!isPlainObject(cursor[key])) {
      throw badRequest('invalidPath', `path ${segments.join('.')} is not writable`);
    }
    cursor = cursor[key];
  }
  return cursor;
}

function applyDirect(attributes, resourceType, path, operation, touched, context) {
  const resolved = resolveDirectPath(resourceType, path);
  touched.add(resolved.root);
  const segments = resolved.segments;
  const leaf = segments[segments.length - 1];

  if (operation.op === 'remove') {
    const parent = existingParent(attributes, segments);
    const key = findKey(parent, leaf);
    if (key === undefined) {
      throw badRequest('invalidPath', `path ${path} does not exist`);
    }
    delete parent[key];
    if (resolved.root === 'members') context.membersRemoved = true;
    return;
  }

  if (operation.value === undefined) {
    throw badRequest('invalidValue', `operation on ${path} requires a value`);
  }
  validateValue(resolved.definition, operation.value, path);

  const located = locate(attributes, segments);
  if (operation.op === 'replace' && !located.found) {
    throw badRequest('invalidPath', `path ${path} does not exist`);
  }

  if (operation.op === 'add' && !located.found) {
    const parent = ensureParent(attributes, segments);
    const key = findKey(parent, leaf) || leaf;
    parent[key] = operation.value;
    return;
  }

  const parent = existingParent(attributes, segments);
  const key = findKey(parent, leaf);
  if (key === undefined) {
    throw badRequest('invalidPath', `path ${path} does not exist`);
  }
  if (operation.op === 'add' && Array.isArray(parent[key])) {
    const additions = Array.isArray(operation.value) ? operation.value : [operation.value];
    parent[key] = parent[key].concat(additions);
    return;
  }
  if (operation.op === 'add' && isPlainObject(parent[key]) && isPlainObject(operation.value)) {
    Object.assign(parent[key], operation.value);
    return;
  }
  parent[key] = operation.value;
}

function applyFiltered(attributes, resourceType, path, operation, touched, context) {
  const resolved = resolveFilteredPath(resourceType, path);
  touched.add(resolved.root);

  const list = fieldValue(attributes, resolved.root);
  if (!Array.isArray(list)) {
    throw badRequest('invalidPath', `path ${path} does not reference a multi-valued attribute`);
  }
  const condition = compileSubtree(parse(resolved.filter));
  const matched = list.filter((item) => matches(item, condition));
  if (matched.length === 0) {
    throw badRequest('invalidPath', `path ${path} matched no element`);
  }

  if (operation.op === 'remove') {
    for (const item of matched) {
      const index = list.indexOf(item);
      if (index !== -1) list.splice(index, 1);
    }
    if (resolved.root === 'members') context.membersRemoved = true;
    return;
  }

  if (operation.value === undefined) {
    throw badRequest('invalidValue', `operation on ${path} requires a value`);
  }
  validateValue(resolved.definition, operation.value, path);

  for (const item of matched) {
    if (Array.isArray(operation.value)) {
      const replacement = operation.value[0];
      if (isPlainObject(replacement)) {
        Object.assign(item, replacement);
      } else {
        item.value = replacement;
      }
      continue;
    }
    if (isPlainObject(operation.value)) {
      Object.assign(item, operation.value);
    } else {
      item.value = operation.value;
    }
  }
}

function applyOperation(attributes, resourceType, operation, touched, context) {
  if (operation.path === null) {
    if (operation.op === 'remove') {
      throw badRequest('invalidPath', 'remove requires a path');
    }
    if (!isPlainObject(operation.value)) {
      throw badRequest('invalidValue', 'operation value must be an object');
    }
    if (!schemaFor(resourceType)) {
      throw badRequest('invalidValue', `unknown resource type ${resourceType}`);
    }
    for (const [key, value] of Object.entries(operation.value)) {
      applyDirect(attributes, resourceType, key, { op: 'replace', value }, touched, context);
    }
    return;
  }

  if (operation.path.indexOf('[') !== -1) {
    applyFiltered(attributes, resourceType, operation.path, operation, touched, context);
    return;
  }
  applyDirect(attributes, resourceType, operation.path, operation, touched, context);
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
  const initialMembers = fieldValue(record.attributes, 'members');
  const hadMembers = resourceType === 'Group'
    && Array.isArray(initialMembers)
    && initialMembers.length > 0;

  const probe = clone(record.attributes);
  const touched = new Set();
  const probeContext = { membersRemoved: false };
  for (const operation of parsed) {
    applyOperation(probe, resourceType, operation, touched, probeContext);
  }

  for (const attribute of touched) {
    assertWritable(actor, resourceType, attribute);
  }

  const next = clone(record.attributes);
  const context = { membersRemoved: false };
  for (const operation of parsed) {
    applyOperation(next, resourceType, operation, touched, context);
  }
  record.attributes = next;

  const members = fieldValue(next, 'members');
  const deleted = resourceType === 'Group'
    && hadMembers
    && context.membersRemoved
    && (!Array.isArray(members) || members.length === 0);
  return { deleted };
}

module.exports = { applyPatch, operationList, readElementFilter };

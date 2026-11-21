'use strict';

const { canRead } = require('../auth/token');
const { sensitiveAttributeOf, USER_SCHEMA, GROUP_SCHEMA, LIST_SCHEMA } = require('./schema');

const ALWAYS_VISIBLE = new Set(['schemas', 'id', 'meta']);

const RESOURCE_SCOPE = { User: 'users', Group: 'groups' };

function clone(value) {
  return value === undefined ? undefined : JSON.parse(JSON.stringify(value));
}

function lowerCaseSet(list) {
  return new Set(list.map((item) => String(item).toLowerCase()));
}

function canReadAttribute(actor, resourceType, attribute) {
  if (ALWAYS_VISIBLE.has(attribute)) return true;
  const sensitive = sensitiveAttributeOf(resourceType, attribute);
  return canRead(actor.scopes, RESOURCE_SCOPE[resourceType], sensitive || 'profile');
}

function trimByScope(actor, resourceType, body) {
  const result = {};
  for (const [key, value] of Object.entries(body)) {
    if (canReadAttribute(actor, resourceType, key)) {
      result[key] = value;
    }
  }
  return result;
}

function applyProjection(body, options) {
  const attributes = Array.isArray(options.attributes) ? lowerCaseSet(options.attributes) : null;
  const excluded = Array.isArray(options.excludedAttributes)
    ? lowerCaseSet(options.excludedAttributes)
    : null;
  if (!attributes && !excluded) return body;

  const result = {};
  for (const [key, value] of Object.entries(body)) {
    const lower = key.toLowerCase();
    if (ALWAYS_VISIBLE.has(key)) {
      result[key] = value;
      continue;
    }
    if (attributes) {
      if (attributes.has(lower)) result[key] = value;
      continue;
    }
    if (!excluded.has(lower)) result[key] = value;
  }
  return result;
}

function baseResource(record, resourceType) {
  const schema = resourceType === 'Group' ? GROUP_SCHEMA : USER_SCHEMA;
  return {
    schemas: [schema],
    id: record.id,
    meta: {
      resourceType,
      created: record.created,
      lastModified: record.lastModified,
      location: `/scim/v2/${record.tenant}/${resourceType === 'Group' ? 'Groups' : 'Users'}/${record.id}`,
      version: String(record.version),
    },
  };
}

function toScimUser(record, actor, options = {}) {
  const full = { ...baseResource(record, 'User'), ...clone(record.attributes) };
  const trimmed = trimByScope(actor, 'User', full);
  return applyProjection(trimmed, options);
}

function toScimGroup(record, actor, options = {}) {
  const full = { ...baseResource(record, 'Group'), ...clone(record.attributes) };
  const trimmed = trimByScope(actor, 'Group', full);
  return applyProjection(trimmed, options);
}

function toListResponse(resources, total, startIndex) {
  return {
    schemas: [LIST_SCHEMA],
    totalResults: total,
    startIndex,
    itemsPerPage: resources.length,
    Resources: resources,
  };
}

module.exports = {
  toScimUser,
  toScimGroup,
  toListResponse,
  trimByScope,
  applyProjection,
  baseResource,
};

'use strict';

const ROUTES = [
  { name: 'users.list', method: 'GET', pattern: /^\/scim\/v2\/([^/]+)\/Users\/?$/, collection: 'Users' },
  { name: 'users.create', method: 'POST', pattern: /^\/scim\/v2\/([^/]+)\/Users\/?$/, collection: 'Users' },
  { name: 'users.get', method: 'GET', pattern: /^\/scim\/v2\/([^/]+)\/Users\/([^/]+)$/, collection: 'Users' },
  { name: 'users.patch', method: 'PATCH', pattern: /^\/scim\/v2\/([^/]+)\/Users\/([^/]+)$/, collection: 'Users' },
  { name: 'users.delete', method: 'DELETE', pattern: /^\/scim\/v2\/([^/]+)\/Users\/([^/]+)$/, collection: 'Users' },
  { name: 'groups.list', method: 'GET', pattern: /^\/scim\/v2\/([^/]+)\/Groups\/?$/, collection: 'Groups' },
  { name: 'groups.create', method: 'POST', pattern: /^\/scim\/v2\/([^/]+)\/Groups\/?$/, collection: 'Groups' },
  { name: 'groups.get', method: 'GET', pattern: /^\/scim\/v2\/([^/]+)\/Groups\/([^/]+)$/, collection: 'Groups' },
  { name: 'groups.patch', method: 'PATCH', pattern: /^\/scim\/v2\/([^/]+)\/Groups\/([^/]+)$/, collection: 'Groups' },
  { name: 'groups.delete', method: 'DELETE', pattern: /^\/scim\/v2\/([^/]+)\/Groups\/([^/]+)$/, collection: 'Groups' },
];

function matchRoute(method, pathname) {
  const normalized = pathname.length > 1 && pathname.endsWith('/')
    ? pathname.slice(0, -1)
    : pathname;
  for (const route of ROUTES) {
    if (route.method !== method) continue;
    const matched = route.pattern.exec(normalized);
    if (!matched) continue;
    return {
      name: route.name,
      collection: route.collection,
      tenant: decodeURIComponent(matched[1]),
      id: matched[2] === undefined ? null : decodeURIComponent(matched[2]),
    };
  }
  return null;
}

function parseEntityTag(value) {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  const withoutWeak = trimmed.startsWith('W/') ? trimmed.slice(2) : trimmed;
  return withoutWeak.replace(/^"|"$/g, '');
}

function parseEntityTags(value) {
  if (typeof value !== 'string') return [];
  const tags = [];
  let i = 0;
  while (i < value.length) {
    while (i < value.length && /[\s,]/.test(value[i])) i += 1;
    if (i >= value.length) break;
    if (value.startsWith('W/', i)) i += 2;
    if (value[i] === '"') {
      i += 1;
      const start = i;
      while (i < value.length && value[i] !== '"') i += 1;
      tags.push(value.slice(start, i));
      if (i < value.length) i += 1;
      continue;
    }
    const start = i;
    while (i < value.length && value[i] !== ',') i += 1;
    tags.push(value.slice(start, i).trim());
  }
  return tags.filter((tag) => tag.length > 0);
}

module.exports = { ROUTES, matchRoute, parseEntityTag, parseEntityTags };

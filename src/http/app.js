'use strict';

const { URL } = require('node:url');
const { verify, bearerToken, canWrite } = require('../auth/token');
const { matchRoute, parseEntityTag } = require('./router');
const { UserRepo } = require('../repo/users');
const { GroupRepo } = require('../repo/groups');
const { parse } = require('../filter/parse');
const { compile } = require('../filter/compile');
const { applyPatch } = require('../patch/apply');
const { toScimUser, toScimGroup, toListResponse } = require('../scim/serialize');
const { sensitiveAttributeOf, isKnownAttribute } = require('../scim/schema');
const {
  ScimError,
  badRequest,
  forbidden,
  notFound,
} = require('../util/errors');

const DEFAULT_COUNT = 100;
const MAX_COUNT = 100;
const RESOURCE_SCOPE = { User: 'users', Group: 'groups' };

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > 1024 * 1024) {
        reject(badRequest('invalidValue', 'request body too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8');
      if (raw.trim().length === 0) {
        resolve(null);
        return;
      }
      try {
        resolve(JSON.parse(raw));
      } catch (err) {
        reject(badRequest('invalidSyntax', 'request body is not valid JSON'));
      }
    });
    req.on('error', reject);
  });
}

function splitList(value) {
  if (value === null || value === undefined || value === '') return null;
  return String(value)
    .split(',')
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function parseProjection(searchParams) {
  const attributes = splitList(searchParams.get('attributes'));
  const excluded = splitList(searchParams.get('excludedAttributes'));
  if (attributes && excluded) {
    throw badRequest('invalidValue', 'attributes and excludedAttributes are mutually exclusive');
  }
  return {
    attributes: attributes || undefined,
    excludedAttributes: excluded || undefined,
  };
}

function parsePositiveInteger(raw, name, fallback) {
  if (raw === null) return fallback;
  if (!/^\d+$/.test(raw)) {
    throw badRequest('invalidValue', `${name} must be an integer`);
  }
  const value = Number(raw);
  if (value < 1) {
    throw badRequest('invalidValue', `${name} must be at least 1`);
  }
  return value;
}

function parsePagination(searchParams) {
  return {
    startIndex: parsePositiveInteger(searchParams.get('startIndex'), 'startIndex', 1),
    count: parsePositiveInteger(searchParams.get('count'), 'count', DEFAULT_COUNT),
  };
}

function normalizeUserName(value) {
  return String(value).trim();
}

function validateCreate(payload, resourceType) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw badRequest('invalidValue', 'request body must be an object');
  }
  if (resourceType === 'User') {
    if (typeof payload.userName !== 'string' || normalizeUserName(payload.userName).length === 0) {
      throw badRequest('invalidValue', 'userName is required');
    }
  } else if (typeof payload.displayName !== 'string' || payload.displayName.length === 0) {
    throw badRequest('invalidValue', 'displayName is required');
  }
}

function pickAttributes(payload) {
  const result = {};
  for (const [key, value] of Object.entries(payload)) {
    if (key === 'schemas' || key === 'id' || key === 'meta') continue;
    result[key] = value;
  }
  return result;
}

function assertCreateScopes(actor, resourceType, payload) {
  for (const key of Object.keys(payload)) {
    if (!isKnownAttribute(resourceType, key)) continue;
    const attribute = sensitiveAttributeOf(resourceType, key) || 'profile';
    if (!canWrite(actor.scopes, RESOURCE_SCOPE[resourceType], attribute)) {
      throw forbidden('insufficientScope', `missing write access to ${key}`);
    }
  }
}

function serialize(record, resourceType, actor, projection) {
  return resourceType === 'Group'
    ? toScimGroup(record, actor, projection)
    : toScimUser(record, actor, projection);
}

function versionOf(record) {
  return String(record.version);
}

function evaluatePrecondition(headerValue, record) {
  if (headerValue === undefined) {
    return { present: false, matched: true, presented: null };
  }
  const presented = parseEntityTag(headerValue);
  return {
    present: true,
    matched: presented === versionOf(record),
    presented,
  };
}

function createApp(options = {}) {
  const app = {
    users: options.users || new UserRepo(),
    groups: options.groups || new GroupRepo(),
    log: options.log || (() => {}),

    async handle(req, res) {
      try {
        const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);
        const actor = verify(bearerToken(req.headers.authorization));
        const route = matchRoute(req.method, url.pathname);
        if (!route) {
          throw notFound('unknown endpoint');
        }
        app.log({
          sub: actor.sub,
          tenant: actor.tenant,
          method: req.method,
          path: url.pathname,
          routeTenant: route.tenant,
        });
        const result = await dispatch(app, route, url, req, actor);
        send(res, result.status, result.body, result.headers);
      } catch (err) {
        const error = err instanceof ScimError
          ? err
          : new ScimError(500, null, err && err.message ? err.message : 'internal error');
        send(res, error.status, error.toBody(), {});
      }
    },
  };
  return app;
}

async function dispatch(app, route, url, req, actor) {
  if (route.collection === 'Users') {
    return handleUsers(app, route, url.searchParams, req, actor);
  }
  return handleGroups(app, route, url.searchParams, req, actor);
}

async function listResources(repo, resourceType, searchParams, actor) {
  const projection = parseProjection(searchParams);
  const { startIndex, count } = parsePagination(searchParams);
  const filterText = searchParams.get('filter');
  let matched;
  if (filterText) {
    const ast = parse(filterText);
    matched = repo.search(compile(ast, actor.tenant));
  } else {
    matched = repo.list(actor.tenant);
  }
  const totalResults = count > MAX_COUNT ? count : matched.length;
  const pageSize = Math.min(count, MAX_COUNT);
  const page = matched.slice(startIndex - 1, startIndex - 1 + pageSize);
  const resources = page.map((row) => serialize(row, resourceType, actor, projection));
  return {
    status: 200,
    body: toListResponse(resources, totalResults, startIndex),
    headers: {},
  };
}

async function handleUsers(app, route, searchParams, req, actor) {
  const repo = app.users;
  if (route.name === 'users.list') {
    return listResources(repo, 'User', searchParams, actor);
  }
  if (route.name === 'users.create') {
    const payload = await readBody(req);
    validateCreate(payload, 'User');
    const attributes = pickAttributes(payload);
    assertCreateScopes(actor, 'User', attributes);
    if (repo.findByUserName(actor.tenant, attributes.userName)) {
      throw new ScimError(409, 'uniqueness', 'userName already exists');
    }
    if (attributes.externalId !== undefined && repo.findByExternalId(actor.tenant, attributes.externalId)) {
      throw new ScimError(409, 'uniqueness', 'externalId already exists');
    }
    const record = repo.create(actor.tenant, attributes);
    return {
      status: 201,
      body: toScimUser(record, actor),
      headers: {
        etag: `"${versionOf(record)}"`,
        location: `/scim/v2/${actor.tenant}/Users/${record.id}`,
      },
    };
  }

  const record = repo.find(actor.tenant, route.id);
  if (!record) {
    throw notFound('user not found');
  }

  if (route.name === 'users.get') {
    return {
      status: 200,
      body: toScimUser(record, actor, parseProjection(searchParams)),
      headers: { etag: `"${versionOf(record)}"` },
    };
  }

  if (route.name === 'users.patch') {
    const guard = evaluatePrecondition(req.headers['if-match'], record);
    if (guard.present && !guard.matched) {
      app.log({
        event: 'version-check',
        sub: actor.sub,
        presented: guard.presented,
        current: versionOf(record),
      });
    }
    const payload = await readBody(req);
    const operations = payload && (payload.Operations || payload.operations);
    applyPatch({ record, resourceType: 'User', operations, actor });
    repo.save(record);
    return {
      status: 200,
      body: toScimUser(record, actor),
      headers: { etag: `"${versionOf(record)}"` },
    };
  }

  if (route.name === 'users.delete') {
    const guard = evaluatePrecondition(req.headers['if-match'], record);
    if (guard.present && !guard.matched) {
      app.log({
        event: 'version-check',
        sub: actor.sub,
        presented: guard.presented,
        current: versionOf(record),
      });
    }
    repo.delete(actor.tenant, record.id);
    return { status: 204, body: null, headers: {} };
  }

  throw notFound('unsupported operation');
}

async function handleGroups(app, route, searchParams, req, actor) {
  const repo = app.groups;
  if (route.name === 'groups.list') {
    return listResources(repo, 'Group', searchParams, actor);
  }
  if (route.name === 'groups.create') {
    const payload = await readBody(req);
    validateCreate(payload, 'Group');
    const attributes = pickAttributes(payload);
    assertCreateScopes(actor, 'Group', attributes);
    const record = repo.create(actor.tenant, attributes);
    return {
      status: 201,
      body: toScimGroup(record, actor),
      headers: {
        etag: `"${versionOf(record)}"`,
        location: `/scim/v2/${actor.tenant}/Groups/${record.id}`,
      },
    };
  }

  const record = repo.find(actor.tenant, route.id);
  if (!record) {
    throw notFound('group not found');
  }

  if (route.name === 'groups.get') {
    return {
      status: 200,
      body: toScimGroup(record, actor, parseProjection(searchParams)),
      headers: { etag: `"${versionOf(record)}"` },
    };
  }

  if (route.name === 'groups.patch') {
    const guard = evaluatePrecondition(req.headers['if-match'], record);
    if (guard.present && !guard.matched) {
      app.log({
        event: 'version-check',
        sub: actor.sub,
        presented: guard.presented,
        current: versionOf(record),
      });
    }
    const payload = await readBody(req);
    const operations = payload && (payload.Operations || payload.operations);
    const outcome = applyPatch({ record, resourceType: 'Group', operations, actor });
    if (outcome.deleted) {
      repo.delete(actor.tenant, record.id);
      return { status: 204, body: null, headers: {} };
    }
    repo.save(record);
    return {
      status: 200,
      body: toScimGroup(record, actor),
      headers: { etag: `"${versionOf(record)}"` },
    };
  }

  if (route.name === 'groups.delete') {
    const guard = evaluatePrecondition(req.headers['if-match'], record);
    if (guard.present && !guard.matched) {
      app.log({
        event: 'version-check',
        sub: actor.sub,
        presented: guard.presented,
        current: versionOf(record),
      });
    }
    repo.delete(actor.tenant, record.id);
    return { status: 204, body: null, headers: {} };
  }

  throw notFound('unsupported operation');
}

function send(res, status, body, headers) {
  const merged = {
    'content-type': 'application/scim+json',
    ...headers,
  };
  res.writeHead(status, merged);
  if (body === null || body === undefined) {
    res.end();
    return;
  }
  res.end(JSON.stringify(body));
}

module.exports = {
  createApp,
  readBody,
  parsePagination,
  parseProjection,
  listResources,
};

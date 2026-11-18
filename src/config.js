import { readFile } from 'node:fs/promises';
import path from 'node:path';

export class ConfigError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ConfigError';
  }
}

const SUPPORTED_TOP_LEVEL_KEYS = new Set([
  'listen',
  'token',
  'defaultRoute',
  'upstreams',
  'routes',
]);
const SUPPORTED_LISTEN_KEYS = new Set(['host', 'port']);
const SUPPORTED_TOKEN_KEYS = new Set([
  'issuer',
  'audience',
  'algorithms',
  'clockToleranceSeconds',
  'jwksPath',
]);
const SUPPORTED_AUTH_KEYS = new Set(['required', 'scopes']);
const SUPPORTED_ROUTE_KEYS = new Set(['id', 'prefix', 'upstream', 'auth']);

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function requireString(value, label) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new ConfigError(`${label} must be a non-empty string`);
  }
  return value;
}

function checkKeys(object, allowed, label) {
  for (const key of Object.keys(object)) {
    if (!allowed.has(key)) {
      throw new ConfigError(`${label} contains unsupported field "${key}"`);
    }
  }
}

function normalizeListen(rawListen) {
  if (rawListen === undefined) {
    return Object.freeze({ host: '127.0.0.1', port: 8080 });
  }
  if (!isPlainObject(rawListen)) {
    throw new ConfigError('listen must be an object');
  }
  checkKeys(rawListen, SUPPORTED_LISTEN_KEYS, 'listen');

  const host = rawListen.host === undefined ? '127.0.0.1' : requireString(rawListen.host, 'listen.host');
  const port = rawListen.port === undefined ? 8080 : rawListen.port;
  if (!Number.isInteger(port) || port < 0 || port > 65535) {
    throw new ConfigError('listen.port must be an integer between 0 and 65535');
  }

  return Object.freeze({ host, port });
}

function normalizeAudience(rawAudience) {
  const values = Array.isArray(rawAudience) ? rawAudience : [rawAudience];
  if (values.length === 0) {
    throw new ConfigError('token.audience must not be empty');
  }
  return Object.freeze(
    values.map((value, index) => requireString(value, `token.audience[${index}]`)),
  );
}

function normalizeToken(rawToken) {
  if (!isPlainObject(rawToken)) {
    throw new ConfigError('token must be an object');
  }
  checkKeys(rawToken, SUPPORTED_TOKEN_KEYS, 'token');

  const issuer = requireString(rawToken.issuer, 'token.issuer');
  const audience = normalizeAudience(rawToken.audience);

  const rawAlgorithms = rawToken.algorithms === undefined ? ['RS256'] : rawToken.algorithms;
  if (!Array.isArray(rawAlgorithms) || rawAlgorithms.length === 0) {
    throw new ConfigError('token.algorithms must be a non-empty array');
  }
  const algorithms = Object.freeze(
    rawAlgorithms.map((value, index) =>
      requireString(value, `token.algorithms[${index}]`),
    ),
  );

  const clockToleranceSeconds =
    rawToken.clockToleranceSeconds === undefined ? 30 : rawToken.clockToleranceSeconds;
  if (!Number.isFinite(clockToleranceSeconds) || clockToleranceSeconds < 0) {
    throw new ConfigError('token.clockToleranceSeconds must be a non-negative number');
  }

  const jwksPath =
    rawToken.jwksPath === undefined
      ? 'keys/jwks.json'
      : requireString(rawToken.jwksPath, 'token.jwksPath');

  return Object.freeze({
    issuer,
    audience,
    algorithms,
    clockToleranceSeconds,
    jwksPath,
  });
}

function normalizeAuth(rawAuth, index) {
  if (!isPlainObject(rawAuth)) {
    throw new ConfigError(`routes[${index}].auth must be an object`);
  }
  checkKeys(rawAuth, SUPPORTED_AUTH_KEYS, `routes[${index}].auth`);

  if (rawAuth.required !== undefined && typeof rawAuth.required !== 'boolean') {
    throw new ConfigError(`routes[${index}].auth.required must be a boolean`);
  }
  const required = rawAuth.required === true;

  const rawScopes = rawAuth.scopes === undefined ? [] : rawAuth.scopes;
  if (!Array.isArray(rawScopes)) {
    throw new ConfigError(`routes[${index}].auth.scopes must be an array`);
  }
  for (const [scopeIndex, scope] of rawScopes.entries()) {
    requireString(scope, `routes[${index}].auth.scopes[${scopeIndex}]`);
  }
  if (required && rawScopes.length === 0) {
    throw new ConfigError(`routes[${index}] requires authentication but lists no scopes`);
  }

  return Object.freeze({
    required,
    scopes: Object.freeze([...rawScopes]),
  });
}

function normalizeRoutes(rawRoutes, upstreams) {
  if (!Array.isArray(rawRoutes) || rawRoutes.length === 0) {
    throw new ConfigError('routes must be a non-empty array');
  }

  const seenIds = new Set();
  return Object.freeze(
    rawRoutes.map((rawRoute, index) => {
      if (!isPlainObject(rawRoute)) {
        throw new ConfigError(`routes[${index}] must be an object`);
      }
      checkKeys(rawRoute, SUPPORTED_ROUTE_KEYS, `routes[${index}]`);

      const id = requireString(rawRoute.id, `routes[${index}].id`);
      if (seenIds.has(id)) {
        throw new ConfigError(`duplicate route id "${id}"`);
      }
      seenIds.add(id);

      const prefix = requireString(rawRoute.prefix, `routes[${index}].prefix`);
      if (!prefix.startsWith('/')) {
        throw new ConfigError(`routes[${index}].prefix must start with "/"`);
      }

      const upstream = requireString(rawRoute.upstream, `routes[${index}].upstream`);
      if (!upstreams.has(upstream)) {
        throw new ConfigError(`routes[${index}] references unknown upstream "${upstream}"`);
      }

      return Object.freeze({
        id,
        prefix,
        upstream,
        auth: normalizeAuth(rawRoute.auth, index),
      });
    }),
  );
}

function normalizeUpstreams(rawUpstreams) {
  if (!isPlainObject(rawUpstreams) || Object.keys(rawUpstreams).length === 0) {
    throw new ConfigError('upstreams must be a non-empty object');
  }

  const upstreams = new Map();
  for (const [name, value] of Object.entries(rawUpstreams)) {
    requireString(name, 'upstream name');
    const base = requireString(value, `upstreams.${name}`);
    let parsed;
    try {
      parsed = new URL(base);
    } catch {
      throw new ConfigError(`upstreams.${name} is not a valid URL`);
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      throw new ConfigError(`upstreams.${name} must use http or https`);
    }
    upstreams.set(name, parsed.origin);
  }

  return upstreams;
}

export function normalizeConfig(raw) {
  if (!isPlainObject(raw)) {
    throw new ConfigError('configuration must be a JSON object');
  }
  checkKeys(raw, SUPPORTED_TOP_LEVEL_KEYS, 'configuration');

  const listen = normalizeListen(raw.listen);
  const token = normalizeToken(raw.token);
  const upstreams = normalizeUpstreams(raw.upstreams);
  const routes = normalizeRoutes(raw.routes, upstreams);

  let defaultRoute = null;
  if (raw.defaultRoute !== undefined && raw.defaultRoute !== null) {
    const defaultId = requireString(raw.defaultRoute, 'defaultRoute');
    defaultRoute = routes.find((route) => route.id === defaultId) ?? null;
    if (defaultRoute === null) {
      throw new ConfigError(`defaultRoute references unknown route "${defaultId}"`);
    }
  }

  return Object.freeze({ listen, token, routes, upstreams, defaultRoute });
}

export async function loadConfig(filePath) {
  let text;
  try {
    text = await readFile(filePath, 'utf8');
  } catch (error) {
    throw new ConfigError(`unable to read configuration at ${filePath}: ${error.message}`);
  }

  let raw;
  try {
    raw = JSON.parse(text);
  } catch (error) {
    throw new ConfigError(`configuration at ${filePath} is not valid JSON: ${error.message}`);
  }

  const config = normalizeConfig(raw);
  return Object.freeze({
    ...config,
    baseDir: path.dirname(path.resolve(filePath)),
  });
}

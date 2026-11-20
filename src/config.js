import { readFileSync } from 'node:fs';
import { dirname, isAbsolute, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
export const DEFAULT_CONFIG_PATH = resolve(HERE, '..', 'config', 'routes.yaml');

/**
 * The route table is a small, fixed-shape YAML document:
 *
 *   server:        { host, port, request_timeout_ms, headers_timeout_ms }
 *   jwt:           { secret, issuer }
 *   introspection: { endpoint, cache_ttl_ms, timeout_ms }
 *   routes:        sequence of { path, methods, auth, scopes, upstream }
 *
 * The reader below handles that subset: nested maps, block sequences, flow
 * sequences of scalars, quoted and bare scalars, and comments.
 */
function parseScalar(raw) {
  const text = raw.trim();
  if (text === '') return null;
  if (text.startsWith('"') && text.endsWith('"') && text.length >= 2) {
    return JSON.parse(text.replace(/\\'/g, "'"));
  }
  if (text.startsWith("'") && text.endsWith("'") && text.length >= 2) {
    return text.slice(1, -1).replace(/''/g, "'");
  }
  if (text.startsWith('[') && text.endsWith(']')) {
    const inner = text.slice(1, -1).trim();
    if (inner === '') return [];
    return inner.split(',').map((part) => parseScalar(part));
  }
  if (text === 'null' || text === '~') return null;
  if (text === 'true') return true;
  if (text === 'false') return false;
  if (/^-?\d+$/.test(text)) return Number.parseInt(text, 10);
  if (/^-?\d*\.\d+$/.test(text)) return Number.parseFloat(text);
  return text;
}

function stripComment(line) {
  let inSingle = false;
  let inDouble = false;
  for (let i = 0; i < line.length; i += 1) {
    const ch = line[i];
    if (ch === "'" && !inDouble) inSingle = !inSingle;
    else if (ch === '"' && !inSingle) inDouble = !inDouble;
    else if (ch === '#' && !inSingle && !inDouble) {
      if (i === 0 || /\s/.test(line[i - 1])) return line.slice(0, i);
    }
  }
  return line;
}

function splitKey(text) {
  const match = /^([A-Za-z0-9_.-]+)\s*:(?:\s+(.*))?$/.exec(text);
  if (!match) return null;
  return { key: match[1], rest: (match[2] ?? '').trim() };
}

export function parseYaml(source) {
  const rows = [];
  for (const rawLine of source.split(/\r?\n/)) {
    const line = stripComment(rawLine);
    if (line.trim() === '') continue;
    const indent = line.length - line.trimStart().length;
    rows.push({ indent, text: line.trim() });
  }

  let cursor = 0;

  function parseBlock(indent) {
    if (cursor >= rows.length) return null;
    if (rows[cursor].text.startsWith('- ') || rows[cursor].text === '-') {
      return parseSequence(indent);
    }
    return parseMapping(indent);
  }

  function parseMapping(indent) {
    const out = {};
    while (cursor < rows.length) {
      const row = rows[cursor];
      if (row.indent < indent) break;
      if (row.indent > indent) {
        throw new Error(`unexpected indentation at "${row.text}"`);
      }
      if (row.text.startsWith('- ')) break;
      const pair = splitKey(row.text);
      if (!pair) throw new Error(`expected "key: value", got "${row.text}"`);
      cursor += 1;
      if (pair.rest !== '') {
        out[pair.key] = parseScalar(pair.rest);
      } else if (cursor < rows.length && rows[cursor].indent > indent) {
        out[pair.key] = parseBlock(rows[cursor].indent);
      } else if (cursor < rows.length && rows[cursor].indent === indent && rows[cursor].text.startsWith('- ')) {
        out[pair.key] = parseSequence(indent);
      } else {
        out[pair.key] = null;
      }
    }
    return out;
  }

  function parseSequence(indent) {
    const out = [];
    while (cursor < rows.length) {
      const row = rows[cursor];
      if (row.indent !== indent) break;
      if (!(row.text.startsWith('- ') || row.text === '-')) break;
      const body = row.text === '-' ? '' : row.text.slice(2).trim();
      cursor += 1;
      if (body === '') {
        out.push(cursor < rows.length && rows[cursor].indent > indent
          ? parseBlock(rows[cursor].indent)
          : null);
        continue;
      }
      const pair = splitKey(body);
      if (pair && !body.startsWith('"') && !body.startsWith("'")) {
        const item = {};
        if (pair.rest !== '') {
          item[pair.key] = parseScalar(pair.rest);
        } else if (cursor < rows.length && rows[cursor].indent > indent) {
          item[pair.key] = parseBlock(rows[cursor].indent);
        } else {
          item[pair.key] = null;
        }
        while (cursor < rows.length && rows[cursor].indent > indent) {
          const childIndent = rows[cursor].indent;
          const child = parseMapping(childIndent);
          Object.assign(item, child);
          if (cursor < rows.length && rows[cursor].indent === childIndent) break;
        }
        out.push(item);
      } else {
        out.push(parseScalar(body));
        if (cursor < rows.length && rows[cursor].indent > indent) {
          throw new Error(`unexpected nested block under "${body}"`);
        }
      }
    }
    return out;
  }

  return parseBlock(rows.length ? rows[0].indent : 0) ?? {};
}

function requireObject(value, name) {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`route table: "${name}" must be a mapping`);
  }
  return value;
}

function requireString(value, name) {
  if (typeof value !== 'string' || value === '') {
    throw new Error(`route table: "${name}" must be a non-empty string`);
  }
  return value;
}

function normalizeRoute(entry, index) {
  const where = `routes[${index}]`;
  const route = requireObject(entry, where);
  const methods = Array.isArray(route.methods)
    ? route.methods.map((m) => requireString(m, `${where}.methods`).toUpperCase())
    : [requireString(route.methods, `${where}.methods`).toUpperCase()];
  const scopes = route.scopes == null
    ? []
    : (Array.isArray(route.scopes) ? route.scopes : [route.scopes]).map((s) => requireString(s, `${where}.scopes`));
  const auth = requireString(route.auth, `${where}.auth`);
  if (!['public', 'jwt', 'introspect'].includes(auth)) {
    throw new Error(`route table: "${where}.auth" must be public, jwt or introspect`);
  }
  return {
    path: requireString(route.path, `${where}.path`),
    methods,
    auth,
    scopes,
    upstream: requireString(route.upstream, `${where}.upstream`).replace(/\/+$/, ''),
  };
}

export function parseConfig(source) {
  const doc = requireObject(parseYaml(source), 'document');
  const server = requireObject(doc.server ?? {}, 'server');
  const jwt = requireObject(doc.jwt ?? {}, 'jwt');
  const introspection = requireObject(doc.introspection ?? {}, 'introspection');
  const routes = Array.isArray(doc.routes) ? doc.routes.map(normalizeRoute) : [];

  return {
    server: {
      host: typeof server.host === 'string' ? server.host : '127.0.0.1',
      port: Number.isInteger(server.port) ? server.port : 8080,
      requestTimeoutMs: Number.isInteger(server.request_timeout_ms) ? server.request_timeout_ms : 15000,
      headersTimeoutMs: Number.isInteger(server.headers_timeout_ms) ? server.headers_timeout_ms : 10000,
    },
    jwt: {
      secret: typeof jwt.secret === 'string' ? jwt.secret : '',
      issuer: typeof jwt.issuer === 'string' ? jwt.issuer : '',
    },
    introspection: {
      endpoint: typeof introspection.endpoint === 'string' ? introspection.endpoint : '',
      cacheTtlMs: Number.isInteger(introspection.cache_ttl_ms) ? introspection.cache_ttl_ms : 30000,
      timeoutMs: Number.isInteger(introspection.timeout_ms) ? introspection.timeout_ms : 2000,
    },
    routes,
  };
}

export function loadConfig(path = process.env.ROUTECLAIM_CONFIG || DEFAULT_CONFIG_PATH) {
  const target = isAbsolute(path) ? path : resolve(process.cwd(), path);
  return parseConfig(readFileSync(target, 'utf8'));
}

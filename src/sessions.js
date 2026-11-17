'use strict';

const { sendJson } = require('./respond');

const SESSION_COOKIE = 'rw_session';

const SESSIONS = Object.freeze({
  sess_alice: Object.freeze({
    user: 'alice',
    tenant: 'acme',
    scopes: Object.freeze(['admin', 'api:read']),
  }),
  sess_bob: Object.freeze({
    user: 'bob',
    tenant: 'acme',
    scopes: Object.freeze([
      'admin',
      'users:read',
      'users:purge',
      'api:read',
      'api:secret',
    ]),
  }),
  sess_carol: Object.freeze({
    user: 'carol',
    tenant: 'acme-internal',
    scopes: Object.freeze(['internal', 'api:read']),
  }),
  sess_dave: Object.freeze({
    user: 'dave',
    tenant: 'acme',
    scopes: Object.freeze(['api:read']),
  }),
});

function safeDecode(value) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function parseCookieHeader(header) {
  const jar = Object.create(null);
  if (typeof header !== 'string' || header.length === 0) {
    return jar;
  }
  for (const part of header.split(';')) {
    const equals = part.indexOf('=');
    if (equals <= 0) {
      continue;
    }
    const name = part.slice(0, equals).trim();
    if (name.length === 0 || name in jar) {
      continue;
    }
    jar[name] = safeDecode(part.slice(equals + 1).trim());
  }
  return jar;
}

function findSession(cookieHeader) {
  const jar = parseCookieHeader(cookieHeader);
  const sessionId = jar[SESSION_COOKIE];
  if (!sessionId || !Object.prototype.hasOwnProperty.call(SESSIONS, sessionId)) {
    return null;
  }
  const record = SESSIONS[sessionId];
  return {
    id: sessionId,
    user: record.user,
    tenant: record.tenant,
    scopes: record.scopes.slice(),
  };
}

function authenticate(req, res, next) {
  const session = findSession(req.headers.cookie);
  if (!session) {
    sendJson(res, 401, { error: 'unauthorized' });
    return;
  }
  req.session = session;
  next();
}

module.exports = {
  SESSION_COOKIE,
  SESSIONS,
  parseCookieHeader,
  findSession,
  authenticate,
};

'use strict';

const crypto = require('node:crypto');
const { unauthorized } = require('../util/errors');

const TOKEN_PREFIX = 'v1';

function secret() {
  return process.env.SCIMFENCE_SECRET || 'scimfence-development-secret';
}

function encode(value) {
  return Buffer.from(value, 'utf8').toString('base64url');
}

function decode(value) {
  return Buffer.from(value, 'base64url').toString('utf8');
}

function signature(payload) {
  return crypto.createHmac('sha256', secret()).update(payload).digest('base64url');
}

function issueToken(claims, options = {}) {
  const ttl = options.ttlSeconds === undefined ? 3600 : options.ttlSeconds;
  const now = Math.floor(Date.now() / 1000);
  const payload = {
    sub: claims.sub || 'test-client',
    tenant: claims.tenant,
    scopes: Array.isArray(claims.scopes) ? claims.scopes.slice() : [],
    iat: now,
    exp: now + ttl,
  };
  const encoded = encode(JSON.stringify(payload));
  return `${TOKEN_PREFIX}.${encoded}.${signature(encoded)}`;
}

function safeEqual(left, right) {
  const a = Buffer.from(left);
  const b = Buffer.from(right);
  if (a.length !== b.length) return false;
  return crypto.timingSafeEqual(a, b);
}

function verify(token) {
  if (typeof token !== 'string' || token.length === 0) {
    throw unauthorized('missing token');
  }
  const parts = token.split('.');
  if (parts.length !== 3 || parts[0] !== TOKEN_PREFIX) {
    throw unauthorized('malformed token');
  }
  const [, encoded, provided] = parts;
  if (!safeEqual(provided, signature(encoded))) {
    throw unauthorized('bad signature');
  }
  let claims;
  try {
    claims = JSON.parse(decode(encoded));
  } catch (err) {
    throw unauthorized('malformed token payload');
  }
  if (!claims || typeof claims.tenant !== 'string' || claims.tenant.length === 0) {
    throw unauthorized('token has no tenant');
  }
  if (!Array.isArray(claims.scopes)) {
    throw unauthorized('token has no scopes');
  }
  if (typeof claims.exp === 'number' && claims.exp < Math.floor(Date.now() / 1000)) {
    throw unauthorized('token expired');
  }
  return {
    sub: claims.sub || null,
    tenant: claims.tenant,
    scopes: claims.scopes.map((scope) => String(scope)),
    exp: claims.exp,
  };
}

function parseScopes(value) {
  if (Array.isArray(value)) {
    return value.map((scope) => String(scope));
  }
  if (typeof value === 'string') {
    return value.split(/[\s,]+/).filter(Boolean);
  }
  return [];
}

function hasScope(scopes, resource, attribute, action) {
  if (!Array.isArray(scopes)) return false;
  const candidates = [
    `attr:${attribute}:${action}`,
    `attr:*:${action}`,
    `${resource}:*:${action}`,
    `${resource}:${attribute}:${action}`,
  ];
  return candidates.some((candidate) => scopes.includes(candidate)) || scopes.includes('*');
}

function canRead(scopes, resource, attribute) {
  return hasScope(scopes, resource, attribute, 'read');
}

function canWrite(scopes, resource, attribute) {
  return hasScope(scopes, resource, attribute, 'write');
}

function bearerToken(headerValue) {
  if (typeof headerValue !== 'string') {
    throw unauthorized('missing authorization header');
  }
  const match = /^Bearer\s+(.+)$/i.exec(headerValue.trim());
  if (!match) {
    throw unauthorized('malformed authorization header');
  }
  return match[1].trim();
}

module.exports = {
  issueToken,
  verify,
  parseScopes,
  hasScope,
  canRead,
  canWrite,
  bearerToken,
};

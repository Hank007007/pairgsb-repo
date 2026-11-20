import { AuditLog } from './audit.js';
import { consume } from './consume.js';
import { issue, pruneExpired } from './issue.js';
import { DEFAULT_PURPOSES, DEFAULT_TTL_MS, MAX_TTL_MS } from './time.js';
import { createStore } from './store.js';

export function createService(options = {}) {
  const {
    store,
    pepper,
    audit = new AuditLog(),
    clock = () => new Date(),
    purposes = DEFAULT_PURPOSES,
    defaultTtlMs = DEFAULT_TTL_MS,
    maxTtlMs = MAX_TTL_MS,
    idFactory,
  } = options;

  if (store === undefined) {
    throw new TypeError('createService requires a store');
  }

  const ctx = {
    store,
    pepper,
    audit,
    clock,
    purposes,
    defaultTtlMs,
    maxTtlMs,
    idFactory,
  };

  return {
    issue: (input) => issue(ctx, input),
    consume: (input) => consume(ctx, input),
    pruneExpired: (now) => pruneExpired(ctx, now),
    audit,
    store,
    pepper,
  };
}

export { AuditLog } from './audit.js';
export { consume } from './consume.js';
export { issue, pruneExpired } from './issue.js';
export { createStore, MemoryStore, FileStore } from './store.js';
export { generateToken, digest, matches } from './token.js';
export { ERROR_CODES, TokenError, isTokenError } from './errors.js';
export { DEFAULT_PURPOSES, DEFAULT_TTL_MS, MAX_TTL_MS } from './time.js';

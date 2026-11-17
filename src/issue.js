import { randomBytes } from 'node:crypto';
import { formatKey, isKnownEnvironment } from './key-format.js';

export class IssueError extends Error {
  constructor(message, code) {
    super(message);
    this.name = 'IssueError';
    this.code = code;
  }
}

export function createIssuer({ store, clock }) {
  function issue({ ownerId, scopes = [], env = 'prod' }) {
    if (typeof ownerId !== 'string' || ownerId.trim() === '') {
      throw new IssueError('ownerId is required', 'invalid_owner');
    }
    if (!isKnownEnvironment(env)) {
      throw new IssueError(`unknown environment: ${env}`, 'invalid_environment');
    }
    if (!Array.isArray(scopes) || scopes.some((scope) => typeof scope !== 'string')) {
      throw new IssueError('scopes must be an array of strings', 'invalid_scopes');
    }

    const id8 = randomBytes(2).toString('hex');
    const secret = randomBytes(32).toString('hex');
    const key = formatKey({ env, id8, secret });
    const now = clock.nowIso();

    const record = {
      id8,
      env,
      ownerId,
      scopes: [...new Set(scopes)],
      key,
      createdAt: now,
      rotatedAt: null,
      graceUntil: null,
      successorId8: null,
      revoked: false,
      revokedAt: null,
    };

    store.put(record);

    return {
      key,
      id8,
      env,
      ownerId,
      scopes: record.scopes,
      createdAt: record.createdAt,
    };
  }

  return { issue };
}

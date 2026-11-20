'use strict';

const fs = require('node:fs');
const path = require('node:path');

const KEY_SEPARATOR = '\0';

function keyFor(tenantId, grantId) {
  return `${tenantId}${KEY_SEPARATOR}${grantId}`;
}

/**
 * Create the set of grant ids that have been withdrawn.
 *
 * The returned object is the public shape every part of the service talks to.
 * Callers never touch the backing representation directly.
 *
 * State is keyed by `(tenantId, grantId)` and, when `filePath` is given,
 * persisted to disk on every change so it survives a process restart.
 *
 * @param {{ filePath?: string }} [options] `filePath` is where revocation
 *   state is kept so that it survives a process restart.
 * @returns {{
 *   revoke(tenantId: string, grantId: string): void,
 *   revokeAll(tenantId: string): void,
 *   isRevoked(tenantId: string, grantId: string): boolean,
 *   generation(tenantId: string): number,
 *   close(): void,
 * }}
 */
function createRevocationStore(options = {}) {
  const filePath = options.filePath ?? null;
  const revoked = new Set();
  const generations = new Map();

  if (filePath !== null) {
    let raw = null;
    try {
      raw = fs.readFileSync(filePath, 'utf8');
    } catch (error) {
      if (error.code !== 'ENOENT') {
        throw error;
      }
    }
    if (raw !== null) {
      const data = JSON.parse(raw);
      for (const key of data.revoked ?? []) {
        revoked.add(key);
      }
      for (const [tenantId, generation] of Object.entries(data.generations ?? {})) {
        generations.set(tenantId, generation);
      }
    }
  }

  function persist() {
    if (filePath === null) {
      return;
    }
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    const payload = JSON.stringify(
      {
        revoked: Array.from(revoked),
        generations: Object.fromEntries(generations),
      },
      null,
      2,
    );
    const temporaryPath = `${filePath}.tmp`;
    fs.writeFileSync(temporaryPath, payload);
    fs.renameSync(temporaryPath, filePath);
  }

  return {
    revoke(tenantId, grantId) {
      if (typeof tenantId !== 'string' || tenantId.length === 0) {
        throw new TypeError('tenantId must be a non-empty string');
      }
      if (typeof grantId !== 'string' || grantId.length === 0) {
        throw new TypeError('grantId must be a non-empty string');
      }
      revoked.add(keyFor(tenantId, grantId));
      persist();
    },

    revokeAll(tenantId) {
      if (typeof tenantId !== 'string' || tenantId.length === 0) {
        throw new TypeError('tenantId must be a non-empty string');
      }
      generations.set(tenantId, (generations.get(tenantId) ?? 0) + 1);
      persist();
    },

    isRevoked(tenantId, grantId) {
      return revoked.has(keyFor(tenantId, grantId));
    },

    generation(tenantId) {
      return generations.get(tenantId) ?? 0;
    },

    close() {
      // nothing to release
    },
  };
}

/**
 * Default on-disk location for revocation state.
 *
 * @param {string} [baseDir]
 * @returns {string}
 */
function defaultRevocationPath(baseDir = process.cwd()) {
  return path.join(baseDir, 'data', 'revocations.json');
}

module.exports = { createRevocationStore, defaultRevocationPath };

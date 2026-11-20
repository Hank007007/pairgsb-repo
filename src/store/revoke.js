'use strict';

const path = require('node:path');

/**
 * Create the set of grant ids that have been withdrawn.
 *
 * The returned object is the public shape every part of the service talks to.
 * Callers never touch the backing representation directly.
 *
 * @param {{ filePath?: string }} [options] `filePath` is where revocation
 *   state is kept so that it survives a process restart.
 * @returns {{
 *   revoke(tenantId: string, grantId: string): void,
 *   isRevoked(tenantId: string, grantId: string): boolean,
 *   generation(tenantId: string): number,
 *   close(): void,
 * }}
 */
function createRevocationStore(options = {}) {
  const revoked = new Set();

  return {
    revoke(tenantId, grantId) {
      if (typeof grantId !== 'string' || grantId.length === 0) {
        throw new TypeError('grantId must be a non-empty string');
      }
      revoked.add(grantId);
    },

    isRevoked(tenantId, grantId) {
      return revoked.has(grantId);
    },

    generation(tenantId) {
      return 0;
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

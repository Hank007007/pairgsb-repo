'use strict';

const { issue } = require('./grant/issue');
const { verify } = require('./grant/verify');
const { createRevocationStore } = require('./store/revoke');

/**
 * Wire the grant lifecycle into a single service object.
 *
 * @param {{
 *   policies: { get(tenantId: string): object | undefined },
 *   store?: ReturnType<typeof createRevocationStore>,
 *   now?: () => number,
 * }} options
 */
function createGrantService(options) {
  const policies = options.policies;
  const store = options.store ?? createRevocationStore();
  const now = options.now ?? Date.now;

  return {
    issue(request) {
      return issue(request, { policies, now, store });
    },

    verify(token, context = {}) {
      return verify(token, {
        ...context,
        now: context.now ?? now(),
        store,
      });
    },

    revoke(tenantId, grantId) {
      store.revoke(tenantId, grantId);
    },

    revokeAll(tenantId) {
      store.revokeAll(tenantId);
    },

    generation(tenantId) {
      return store.generation(tenantId);
    },

    close() {
      store.close();
    },
  };
}

module.exports = { createGrantService };

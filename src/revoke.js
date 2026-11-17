export function createRevoker({ store, clock, cache }) {
  function revoke(id8) {
    const current = store.getById(id8);
    if (!current) {
      return { found: false, id8, status: 'unknown' };
    }
    if (current.revoked) {
      return {
        found: true,
        id8,
        status: 'revoked',
        revokedAt: current.revokedAt,
        alreadyRevoked: true,
      };
    }

    const revokedAt = clock.nowIso();
    store.update(id8, { revoked: true, revokedAt });
    cache.delete(id8);

    return {
      found: true,
      id8,
      status: 'revoked',
      revokedAt,
      alreadyRevoked: false,
    };
  }

  return { revoke };
}

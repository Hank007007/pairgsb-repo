import { IssueError } from './issue.js';

export function createRotator({ store, clock, graceMs, issuer, cache }) {
  function rotate(id8) {
    const current = store.getById(id8);
    if (!current) {
      throw new IssueError(`unknown key: ${id8}`, 'unknown_key');
    }
    if (current.revoked) {
      throw new IssueError(`key is revoked: ${id8}`, 'revoked');
    }

    const rotatedAt = clock.nowIso();
    const graceUntil = new Date(clock.now() + graceMs).toISOString();

    const issued = issuer.issue({
      ownerId: current.ownerId,
      scopes: current.scopes,
      env: current.env,
    });

    store.update(id8, {
      rotatedAt,
      graceUntil,
      successorId8: issued.id8,
    });
    cache.delete(id8);

    return {
      ...issued,
      previousId8: id8,
      graceUntil,
    };
  }

  return { rotate };
}

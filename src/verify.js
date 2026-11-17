import { parseKey } from './key-format.js';
import { hashSecret, digestsMatch } from './digest.js';

export function createVerifier({ store, cache, logger, pepper, clock }) {
  function verify(presentedKey, { env } = {}) {
    const parsed = parseKey(presentedKey);
    if (!parsed) {
      return { ok: false, reason: 'invalid_format' };
    }

    const actualDigest = hashSecret(pepper, parsed.secret);
    const record = cache.peek(parsed.id8) ?? store.peekById(parsed.id8);
    const digestMatches = digestsMatch(actualDigest, record?.secretHash);
    if (!record || !digestMatches) {
      return { ok: false, reason: 'unknown_key' };
    }

    if (parsed.env !== record.env || parsed.env !== env) {
      logger.warn('key presented under a different environment; rejected', {
        keyEnv: parsed.env,
        requestEnv: env,
        id8: parsed.id8,
      });
      return { ok: false, reason: 'env_mismatch', id8: record.id8 };
    }

    if (record.revoked) {
      return { ok: false, reason: 'revoked', id8: record.id8 };
    }

    if (record.graceUntil && clock.now() >= Date.parse(record.graceUntil)) {
      return { ok: false, reason: 'grace_expired', id8: record.id8 };
    }

    cache.set(parsed.id8, record);

    return {
      ok: true,
      id8: record.id8,
      ownerId: record.ownerId,
      scopes: [...record.scopes],
      env: record.env,
    };
  }

  return { verify };
}

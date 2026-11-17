import { parseKey } from './key-format.js';

export function createVerifier({ store, cache, logger }) {
  function verify(presentedKey, { env } = {}) {
    const parsed = parseKey(presentedKey);
    if (!parsed) {
      return { ok: false, reason: 'invalid_format' };
    }

    const record = cache.get(parsed.id8) ?? store.lookup(presentedKey);
    if (!record || record.key !== presentedKey) {
      return { ok: false, reason: 'unknown_key' };
    }
    cache.set(parsed.id8, record);

    if (env && parsed.env !== env) {
      logger.warn('key presented under a different environment', {
        keyEnv: parsed.env,
        requestEnv: env,
        id8: parsed.id8,
      });
    }

    if (record.revoked) {
      return { ok: false, reason: 'revoked', id8: record.id8 };
    }

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

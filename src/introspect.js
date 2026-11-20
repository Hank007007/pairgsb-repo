/**
 * Token introspection.
 *
 * The authorization service answers POST /introspect with a JSON document of
 * the shape { active, sub, scope, exp?, token_id? }. The gateway keeps the
 * answers it receives so that repeated calls with the same token do not each
 * cost a round trip.
 */

/**
 * Ask the authorization service about one token.
 *
 * @returns {Promise<{ok: true, payload: object} | {ok: false, reason: string}>}
 */
export async function introspectRemote(token, options = {}) {
  const endpoint = options.endpoint;
  if (typeof endpoint !== 'string' || endpoint === '') {
    return { ok: false, reason: 'not-configured' };
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), options.timeoutMs ?? 2000);
  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        accept: 'application/json',
      },
      body: new URLSearchParams({ token }).toString(),
      signal: controller.signal,
    });
    if (!response.ok) {
      return { ok: false, reason: `status-${response.status}` };
    }
    const payload = await response.json();
    if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) {
      return { ok: false, reason: 'malformed-response' };
    }
    if (typeof payload.active !== 'boolean') {
      return { ok: false, reason: 'malformed-response' };
    }
    return { ok: true, payload };
  } catch (error) {
    return { ok: false, reason: error && error.name === 'AbortError' ? 'timeout' : 'transport-error' };
  } finally {
    clearTimeout(timer);
  }
}

export function createIntrospector(options = {}) {
  const cache = new Map();
  const now = options.now ?? (() => Date.now());
  const ttlMs = Number.isInteger(options.cacheTtlMs) ? options.cacheTtlMs : 30000;

  function keyFor(token) {
    return `${options.endpoint ?? ''} ${token}`;
  }

  /**
   * A record lives for the smaller of cache_ttl_ms and the token's own
   * exp, per docs/route-auth.md section 5.1.
   */
  function expiryFor(payload) {
    let expiresAt = now() + ttlMs;
    if (payload && typeof payload.exp === 'number') {
      expiresAt = Math.min(expiresAt, payload.exp * 1000);
    }
    return expiresAt;
  }

  /**
   * Look up one token.
   *
   * @returns {Promise<{ok: true, payload: object} | {ok: false, reason: string}>}
   */
  async function lookup(token) {
    const key = keyFor(token);
    const cached = cache.get(key);
    if (cached !== undefined) {
      if (cached.expiresAt > now()) return cached.result;
      cache.delete(key);
    }
    const result = await introspectRemote(token, options);
    // Failures (transport error, timeout, non-2xx, malformed body) are
    // never cached: the next request with this token introspects again.
    if (!result.ok) return result;
    // Both active and inactive decisions are definitive for this request
    // and are retained for the same bounded window.
    cache.set(key, { result, expiresAt: expiryFor(result.payload) });
    return result;
  }

  function clear(token) {
    if (token === undefined) cache.clear();
    else cache.delete(keyFor(token));
  }

  function size() {
    return cache.size;
  }

  return { lookup, clear, size };
}

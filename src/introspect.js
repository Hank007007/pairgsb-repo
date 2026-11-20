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

  async function query(token) {
    return introspectRemote(token, options);
  }

  /**
   * Look up one token.
   *
   * @returns {Promise<{ok: true, payload: object} | {ok: false, reason: string}>}
   */
  async function lookup(token) {
    const cached = cache.get(token);
    if (cached !== undefined) {
      return cached;
    }
    const first = await query(token);
    if (!first.ok) {
      const record = { ok: false, reason: first.reason };
      cache.set(token, record);
      return record;
    }
    if (first.payload.active === false) {
      const second = await query(token);
      if (!second.ok) {
        const record = { ok: false, reason: second.reason };
        cache.set(token, record);
        return record;
      }
      const record = { ok: true, payload: second.payload };
      cache.set(token, record);
      return record;
    }
    const record = { ok: true, payload: first.payload };
    cache.set(token, record);
    return record;
  }

  function clear(token) {
    if (token === undefined) cache.clear();
    else cache.delete(token);
  }

  function size() {
    return cache.size;
  }

  return { lookup, clear, size };
}

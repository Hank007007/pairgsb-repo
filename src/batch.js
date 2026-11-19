function cacheKey(request) {
  return JSON.stringify([
    request.principal,
    request.resource,
    request.action,
    request.context ?? null,
  ]);
}

/**
 * Evaluate a batch of requests through an engine.
 *
 * Each element of the returned array corresponds positionally to the request
 * at the same index.
 *
 * @param {import('./engine.js').PolicyEngine} engine
 * @param {Array<object>} requests
 * @param {import('./cache.js').TtlCache | null | undefined} cache
 * @returns {Array<{ allow: boolean, matchedPolicy: string | null, reason: string }>}
 */
export function decideMany(engine, requests, cache) {
  if (!Array.isArray(requests)) {
    throw new TypeError('decideMany expects an array of requests');
  }

  const results = new Array(requests.length);

  for (let index = 0; index < requests.length; index += 1) {
    const request = requests[index];

    if (cache === null || cache === undefined) {
      results[index] = engine.decide(request);
      continue;
    }

    const key = cacheKey(request);
    const hit = cache.get(key);
    if (hit !== undefined) {
      results[index] = { ...hit };
      continue;
    }

    const decision = engine.decide(request);
    cache.set(key, decision);
    results[index] = { ...decision };
  }

  return results;
}

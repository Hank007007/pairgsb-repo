/**
 * Decision cache.
 *
 * Stores the outcome of `check` calls. Entries are keyed by the request
 * triple and carry the metadata needed to decide whether the entry still
 * describes the current world: tenant, snapshot version, and the namespace
 * versions the decision was computed under.
 */

import { isFresh } from '../store/consistency.js';

export function createDecisionCache(options = {}) {
  return {
    entries: new Map(),
    limit: options.limit ?? 10_000,
  };
}

/**
 * Build the cache key for a decision.
 *
 * @param {{tenant: string, object: object, relation: string, subject: object, contextual?: string[]}} request
 */
export function decisionKey(request) {
  const parts = [
    request.tenant ?? '',
    `${request.object.type}:${request.object.id}`,
    request.relation,
    `${request.subject.type}:${request.subject.id}`,
  ];
  if (request.contextual && request.contextual.length > 0) {
    parts.push([...request.contextual].sort().join(','));
  }
  return parts.join('|');
}

export function getDecision(cache, key) {
  return cache.entries.get(key);
}

export function putDecision(cache, key, entry) {
  if (cache.entries.size >= cache.limit) {
    const oldest = cache.entries.keys().next();
    if (!oldest.done) cache.entries.delete(oldest.value);
  }
  cache.entries.set(key, entry);
}

export function clearDecisions(cache) {
  cache.entries.clear();
}

export { isFresh };

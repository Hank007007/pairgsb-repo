/**
 * Scope handling.
 *
 * A token's scopes arrive as either a space separated string or an array,
 * under `scope` or `scopes`. Routes list the scopes they require.
 */

/** Read the scope list out of a token payload. */
export function readScopes(claims) {
  if (!claims || typeof claims !== 'object') return [];
  const raw = claims.scope !== undefined ? claims.scope : claims.scopes;
  if (raw === undefined || raw === null) return [];
  if (Array.isArray(raw)) {
    return raw.filter((item) => typeof item === 'string' && item !== '');
  }
  if (typeof raw === 'string') {
    return raw.split(/\s+/).filter((item) => item !== '');
  }
  return [];
}

/** True when the granted scope covers the required scope. */
export function hasScope(granted, required) {
  return String(granted).startsWith(String(required));
}

/** True when every required scope is covered by the granted list. */
export function hasScopes(granted, required) {
  const held = Array.isArray(granted) ? granted : readScopes(granted);
  if (!required || required.length === 0) return true;
  return required.every((needed) => held.some((have) => hasScope(have, needed)));
}

/** Space separated, de-duplicated form used on the wire. */
export function formatScopes(scopes) {
  const seen = new Set();
  for (const scope of scopes || []) {
    if (typeof scope === 'string' && scope !== '') seen.add(scope);
  }
  return [...seen].join(' ');
}

/**
 * Scope evaluation helpers.
 *
 * A route lists the scopes a caller must hold; the session token carries the
 * scopes granted to the caller. Scopes are namespaced with colons, for example
 * `orders:read` or `admin:export`.
 */

/**
 * Reads the `scope` claim, which may be a JSON array of strings or a
 * space-delimited string.
 */
export function parseScopes(payload) {
  if (payload === null || typeof payload !== 'object') {
    return [];
  }

  const raw = payload.scope;
  if (Array.isArray(raw)) {
    return raw.filter((scope) => typeof scope === 'string' && scope.length > 0);
  }
  if (typeof raw === 'string') {
    return raw.split(/\s+/).filter((scope) => scope.length > 0);
  }
  return [];
}

export function hasScope(grantedScopes, requiredScope) {
  if (!Array.isArray(grantedScopes) || typeof requiredScope !== 'string' || requiredScope === '') {
    return false;
  }

  return grantedScopes.some(
    (scope) => typeof scope === 'string' && scope === requiredScope,
  );
}

/**
 * Returns the subset of `requiredScopes` the caller does not hold.
 */
export function missingScopes(grantedScopes, requiredScopes) {
  if (!Array.isArray(requiredScopes)) {
    return [];
  }

  return requiredScopes.filter((scope) => !hasScope(grantedScopes, scope));
}

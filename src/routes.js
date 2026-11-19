const HEX_ESCAPE = /^[0-9A-Fa-f]{2}$/;

/**
 * Normalizes the path component of a request target per gateway-authz §2.1:
 * the query string and fragment are excluded, percent-encoded octets are
 * decoded exactly once, dot segments are resolved (clamped at the root) and
 * consecutive slashes collapse to one.
 *
 * Returns the normalized absolute path, or `null` when the target is not an
 * absolute path or carries a malformed percent escape.
 */
export function normalizeRequestPath(requestTarget) {
  if (typeof requestTarget !== 'string') {
    return null;
  }

  let end = requestTarget.length;
  const queryIndex = requestTarget.indexOf('?');
  const fragmentIndex = requestTarget.indexOf('#');
  if (queryIndex !== -1) {
    end = Math.min(end, queryIndex);
  }
  if (fragmentIndex !== -1) {
    end = Math.min(end, fragmentIndex);
  }
  const rawPath = requestTarget.slice(0, end);

  if (!rawPath.startsWith('/')) {
    return null;
  }

  for (let index = 0; index < rawPath.length; index += 1) {
    if (rawPath[index] === '%') {
      if (!HEX_ESCAPE.test(rawPath.slice(index + 1, index + 3))) {
        return null;
      }
      index += 2;
    }
  }

  let decoded;
  try {
    decoded = decodeURIComponent(rawPath);
  } catch {
    return null;
  }

  const segments = [];
  for (const segment of decoded.split('/')) {
    if (segment === '' || segment === '.') {
      continue;
    }
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }

  return `/${segments.join('/')}`;
}

/**
 * Resolves a normalized request path to a route entry.
 *
 * The route table is evaluated in declaration order and the first route whose
 * prefix matches on a path segment boundary wins. When no route matches, the
 * configured default route is returned; `null` means the request is not
 * routable.
 */
export function matchRoute(normalizedPath, routes, defaultRoute = null) {
  if (typeof normalizedPath !== 'string') {
    return defaultRoute;
  }

  for (const route of routes) {
    const prefix = route.prefix;
    if (normalizedPath === prefix) {
      return route;
    }
    const boundary = prefix.endsWith('/') ? prefix : `${prefix}/`;
    if (normalizedPath.startsWith(boundary)) {
      return route;
    }
  }

  return defaultRoute;
}

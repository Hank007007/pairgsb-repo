/**
 * Route selection.
 *
 * The route table is consulted for every inbound request. A route covers a
 * request when the request's normalized path equals the route's path or
 * begins with it on a segment boundary. See docs/route-auth.md sections 1-3.
 */

/** Strip the query string from a request target. */
export function requestPath(url) {
  const raw = typeof url === 'string' && url !== '' ? url : '/';
  const queryAt = raw.indexOf('?');
  const path = queryAt === -1 ? raw : raw.slice(0, queryAt);
  return path === '' ? '/' : path;
}

/**
 * Normalize a request path per docs/route-auth.md section 1:
 * percent-decode exactly once, drop empty and `.` segments, resolve `..`
 * (clamped at the root), and remove any trailing slash.
 *
 * @returns {string|null} the normalized path, or null when the path cannot
 *          be decoded
 */
export function normalizePath(path) {
  let decoded;
  try {
    decoded = decodeURIComponent(path);
  } catch {
    return null;
  }
  const segments = [];
  for (const segment of decoded.replace(/\\/g, '/').split('/')) {
    if (segment === '' || segment === '.') continue;
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  return `/${segments.join('/')}`;
}

/** Lowercased route path without trailing slashes, for comparison. */
function routeKey(route) {
  const key = route.path.replace(/\/+$/, '').toLowerCase();
  return key === '' ? '/' : key;
}

/**
 * Pick the route for a normalized path.
 *
 * A route matches when the path equals the route's path or begins with it
 * followed by `/`; comparison is case-insensitive. When several routes
 * match, the longest route path wins. Declaration order is not significant.
 */
export function resolve(routes, path) {
  const wanted = path.toLowerCase();
  let best = null;
  let bestLength = -1;
  for (const route of routes) {
    const base = routeKey(route);
    const covers = wanted === base
      || (base === '/' ? wanted.startsWith('/') : wanted.startsWith(`${base}/`));
    if (covers && base.length > bestLength) {
      best = route;
      bestLength = base.length;
    }
  }
  return best;
}

/**
 * Match a request against the route table.
 *
 * @returns {{status: 'matched', path: string, route: object}
 *         | {status: 'bad-path'}
 *         | {status: 'no-route', path: string}
 *         | {status: 'method-not-allowed', path: string, route: object}}
 */
export function matchRoute(routes, req) {
  const path = normalizePath(requestPath(req && req.url));
  if (path === null) return { status: 'bad-path' };
  const route = resolve(routes, path);
  if (!route) return { status: 'no-route', path };
  const method = req && typeof req.method === 'string' ? req.method : 'GET';
  const accepted = route.methods.some((listed) => listed === '*' || listed === method.toUpperCase());
  if (!accepted) {
    return { status: 'method-not-allowed', path, route };
  }
  return { status: 'matched', path, route };
}

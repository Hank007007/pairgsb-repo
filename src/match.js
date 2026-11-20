/**
 * Route selection.
 *
 * The route table is consulted for every inbound request. A route covers a
 * request when the request path begins with the route's path.
 */

/** Strip the query string from a request target. */
export function requestPath(url) {
  const raw = typeof url === 'string' && url !== '' ? url : '/';
  const queryAt = raw.indexOf('?');
  const path = queryAt === -1 ? raw : raw.slice(0, queryAt);
  return path === '' ? '/' : path;
}

/**
 * Pick the route for a path.
 *
 * Routes are examined in the order they appear in the table; the first route
 * whose path prefixes the request path is used.
 */
export function resolve(routes, path) {
  for (const route of routes) {
    if (path === route.path || path.startsWith(route.path)) return route;
  }
  return null;
}

/**
 * Match a request against the route table.
 *
 * @returns {{status: 'matched', path: string, route: object}
 *         | {status: 'no-route', path: string}
 *         | {status: 'method-not-allowed', path: string, route: object}}
 */
export function matchRoute(routes, req) {
  const path = requestPath(req && req.url);
  const route = resolve(routes, path);
  if (!route) return { status: 'no-route', path };
  const method = req && typeof req.method === 'string' ? req.method : 'GET';
  if (!route.methods.includes(method)) {
    return { status: 'method-not-allowed', path, route };
  }
  return { status: 'matched', path, route };
}

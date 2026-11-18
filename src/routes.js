/**
 * Resolves an incoming request URL to a route entry.
 *
 * The route table is evaluated in declaration order and the first route whose
 * prefix matches wins. When no route matches, the configured default route is
 * returned; `null` means the request is not routable.
 */
export function matchRoute(requestUrl, routes, defaultRoute = null) {
  if (typeof requestUrl !== 'string') {
    return defaultRoute;
  }

  for (const route of routes) {
    if (requestUrl.startsWith(route.prefix)) {
      return route;
    }
  }

  return defaultRoute;
}

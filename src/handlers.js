'use strict';

const { sendJson } = require('./respond');

function routeId(req) {
  return req.route ? req.route.id : null;
}

const ROUTES = [
  { method: 'GET', path: '/health', handler: health },
  { method: 'GET', path: '/admin/users', handler: adminUsers },
  { method: 'DELETE', path: '/admin/users', handler: adminDeleteUsers },
  { method: 'GET', path: '/admin/audit', handler: adminAudit },
  { method: 'GET', path: '/admin/overview', handler: adminOverview },
  { method: 'GET', path: '/api/items', handler: apiItems },
  { method: 'GET', path: '/api/orders/secret', handler: apiSecret },
  { method: 'GET', path: '/api/a/b/secret', handler: apiNestedSecret },
  { method: 'GET', path: '/internal/reports', handler: internalReports },
];

function health(req, res) {
  sendJson(res, 200, { status: 'ok', route: routeId(req) });
}

function adminUsers(req, res) {
  sendJson(res, 200, {
    handler: 'admin.users.list',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function adminDeleteUsers(req, res) {
  sendJson(res, 200, {
    handler: 'admin.users.purge',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function adminAudit(req, res) {
  sendJson(res, 200, {
    handler: 'admin.audit.read',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function adminOverview(req, res) {
  sendJson(res, 200, {
    handler: 'admin.overview',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function apiItems(req, res) {
  sendJson(res, 200, {
    handler: 'api.items.list',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function apiSecret(req, res) {
  sendJson(res, 200, {
    handler: 'api.orders.secret',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function internalReports(req, res) {
  sendJson(res, 200, {
    handler: 'internal.reports',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}

function apiNestedSecret(req, res) {
  sendJson(res, 200, {
    handler: 'api.nested.secret',
    tenant: req.session.tenant,
    route: routeId(req),
  });
}


function matchHandler(method, pathname) {  for (const route of ROUTES) {
    if (route.method === method && route.path === pathname) {
      return route.handler;
    }
  }
  return null;
}

function dispatchToHandler(req, res) {
  const handler = matchHandler(req.method, req.path);
  if (!handler) {
    sendJson(res, 404, { error: 'not_found' });
    return;
  }
  handler(req, res);
}

module.exports = { ROUTES, matchHandler, dispatchToHandler };

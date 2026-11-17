'use strict';

const OVERRIDE_HEADER = 'x-http-method-override';

/**
 * Applies the client-supplied method override to the request so downstream
 * routing sees the effective method. Returns the method that was applied, or
 * null when the header was absent or unusable.
 */
function applyMethodOverride(req) {
  const header = req.headers[OVERRIDE_HEADER];
  if (typeof header !== 'string' || header.length === 0) {
    return null;
  }

  const method = header.trim().toUpperCase();
  if (method.length === 0) {
    return null;
  }

  req.originalMethod = req.method;
  req.method = method;
  req.methodOverridden = true;
  return method;
}

function createDispatch() {
  return function dispatch(req, res, next) {
    applyMethodOverride(req);
    next();
  };
}

module.exports = { OVERRIDE_HEADER, applyMethodOverride, createDispatch };

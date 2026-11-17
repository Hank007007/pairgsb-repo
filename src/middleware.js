'use strict';

const { normalizePath } = require('./normalize');
const { selectRule, missingScopes } = require('./rules');
const { sendJson } = require('./respond');

function createAuthorize(rules) {
  return function authorize(req, res, next) {
    const pathname = normalizePath(req.url);
    req.path = pathname;

    const rule = selectRule(rules, req.session, req.method, req.url);
    if (!rule) {
      next();
      return;
    }

    const missing = missingScopes(rule, req.session);
    if (missing.length > 0) {
      sendJson(res, 403, {
        error: 'forbidden',
        rule: rule.id,
        missing,
      });
      return;
    }

    req.route = rule;
    next();
  };
}

module.exports = { createAuthorize };

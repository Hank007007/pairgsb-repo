'use strict';

const { PathNormalizationError, normalizePath } = require('./normalize');
const { selectRule, missingScopes } = require('./rules');
const { sendJson } = require('./respond');

function createAuthorize(rules) {
  return function authorize(req, res, next) {
    let pathname;
    try {
      pathname = normalizePath(req.url);
    } catch (error) {
      if (error instanceof PathNormalizationError) {
        sendJson(res, 400, { error: 'bad_request' });
        return;
      }
      throw error;
    }

    req.path = pathname;

    const rule = selectRule(rules, req.session, req.method, pathname);
    if (!rule) {
      sendJson(res, 403, { error: 'forbidden' });
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

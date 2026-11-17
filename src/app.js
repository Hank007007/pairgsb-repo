'use strict';

const { authenticate } = require('./sessions');
const { createAuthorize } = require('./middleware');
const { createDispatch } = require('./dispatch');
const { dispatchToHandler } = require('./handlers');
const { loadRules } = require('./rules');
const { sendJson } = require('./respond');

function compose(middlewares, terminal) {
  return function run(req, res) {
    let index = -1;

    function next(err) {
      index += 1;
      if (err) {
        sendJson(res, 500, { error: 'internal_error' });
        return;
      }
      if (index >= middlewares.length) {
        terminal(req, res);
        return;
      }
      const middleware = middlewares[index];
      try {
        middleware(req, res, next);
      } catch {
        sendJson(res, 500, { error: 'internal_error' });
      }
    }

    next();
  };
}

function createApp(options = {}) {
  const rules = options.rules || loadRules(options.routesFile);

  return compose(
    [authenticate, createDispatch(), createAuthorize(rules)],
    dispatchToHandler,
  );
}

module.exports = { createApp, compose };

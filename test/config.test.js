import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadConfig, normalizeConfig, ConfigError } from '../src/config.js';
import { projectRoot } from '../test-support/helpers.js';

test('loads the shipped route table', async () => {
  const config = await loadConfig(path.join(projectRoot, 'config', 'routes.json'));

  assert.equal(config.defaultRoute.id, 'public-site');
  assert.equal(config.routes.length, 4);
  assert.equal(config.upstreams.get('orders'), 'http://127.0.0.1:9211');

  const orders = config.routes.find((route) => route.id === 'orders');
  assert.deepEqual(orders.auth.scopes, ['orders:read']);
  assert.equal(orders.auth.required, true);

  const publicSite = config.routes.find((route) => route.id === 'public-site');
  assert.equal(publicSite.auth.required, false);
  assert.deepEqual(publicSite.auth.scopes, []);
});

test('rejects a route that points at an unknown upstream', () => {
  assert.throws(
    () =>
      normalizeConfig({
        token: { issuer: 'i', audience: 'a' },
        upstreams: { web: 'http://127.0.0.1:9000' },
        routes: [
          {
            id: 'web',
            prefix: '/web',
            upstream: 'missing',
            auth: { required: false },
          },
        ],
      }),
    (error) => error instanceof ConfigError && /unknown upstream/.test(error.message),
  );
});

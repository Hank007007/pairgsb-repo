import test from 'node:test';
import assert from 'node:assert/strict';

import {
  registered_domains,
  resolve_endpoint,
} from '../src/report/endpoint.mjs';

test('resolves an endpoint on a registered platform domain', () => {
  assert.equal(
    resolve_endpoint('https://reports.platform.example/csp/collect'),
    'https://reports.platform.example/csp/collect',
  );
});

test('lists the domains registered for a tenant', () => {
  assert.deepEqual(registered_domains('tenant-a'), [
    'platform.example',
    'tenant-a.example',
  ]);
});

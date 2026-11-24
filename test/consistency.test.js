import test from 'node:test';
import assert from 'node:assert/strict';

import { isFresh } from '../src/store/consistency.js';

const entry = {
  tenant: 'acme',
  snapshotVersion: 5,
  namespaceVersions: { doc: 2 },
};

test('isFresh requires the same tenant, a new enough snapshot and matching namespace versions', () => {
  assert.equal(isFresh(entry, { tenant: 'acme', minVersion: 5, namespaceVersions: { doc: 2 } }), true);
  assert.equal(isFresh(entry, { tenant: 'acme', minVersion: 4, namespaceVersions: { doc: 2 } }), true);
  assert.equal(isFresh(entry, { tenant: 'acme', minVersion: 6, namespaceVersions: { doc: 2 } }), false);
  assert.equal(isFresh(entry, { tenant: 'acme', minVersion: 5, namespaceVersions: { doc: 3 } }), false);
  assert.equal(isFresh(entry, { tenant: 'other', minVersion: 5, namespaceVersions: { doc: 2 } }), false);
  assert.equal(isFresh(undefined, { tenant: 'acme', minVersion: 0, namespaceVersions: {} }), false);
});

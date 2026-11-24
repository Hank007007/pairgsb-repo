import test from 'node:test';
import assert from 'node:assert/strict';

import { makeService } from './helpers.js';

test('a direct tuple grants the relation to its subject', () => {
  const service = makeService();
  service.write('doc:1#reader@user:alice');

  assert.equal(service.check('doc:1', 'reader', 'user:alice'), true);
  assert.equal(service.check('doc:1', 'reader', 'user:bob'), false);
  assert.equal(service.check('doc:2', 'reader', 'user:alice'), false);
});

test('a one-level userset subject is resolved', () => {
  const service = makeService();
  service.write('group:eng#member@user:alice');
  service.write('doc:1#reader@group:eng#member');

  assert.equal(service.check('doc:1', 'reader', 'user:alice'), true);
  assert.equal(service.check('doc:1', 'reader', 'user:bob'), false);
});

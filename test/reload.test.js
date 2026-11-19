import test from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';

import { createAcl, reloadAcl } from '../src/acl/reload.js';

const baseRules = {
  principals: {
    'user:alice': {
      rules: [
        { action: 'publish', topic: 'sensors/a', effect: 'allow' },
        { action: 'subscribe', topic: 'sensors/a', effect: 'allow' },
      ],
    },
  },
};

test('createAcl starts at version 0 with the supplied rules', () => {
  const acl = createAcl({ rules: baseRules });
  assert.equal(acl.version, 0);
  assert.equal(acl.rules.principals['user:alice'].rules.length, 2);
});

test('publish and subscribe delegate to the same decision path', () => {
  const acl = createAcl({ rules: baseRules });
  const identity = { username: 'alice' };

  assert.equal(acl.publish(identity, 'sensors/a').allowed, true);
  assert.equal(acl.publish(identity, 'farm/a').allowed, false);
  assert.equal(acl.subscribe(identity, 'sensors/a').allowed, true);
  assert.equal(acl.canReceive(identity, 'sensors/a').allowed, true);
});

test('replaceRules bumps the version and swaps the table', () => {
  const acl = createAcl({ rules: baseRules });
  const version = acl.replaceRules({ principals: {} });

  assert.equal(version, 1);
  assert.equal(acl.version, 1);
  assert.equal(acl.publish({ username: 'alice' }, 'sensors/a').allowed, false);
});

test('reloadAcl loads a directory and installs it', async () => {
  const acl = createAcl({ rules: { principals: {} } });
  const directory = fileURLToPath(new URL('../acl/', import.meta.url));
  const { version } = await reloadAcl(acl, directory, {
    loadedAt: '2025-01-01T00:00:00.000Z',
  });

  assert.equal(version, 1);
  assert.equal(acl.loadedAt, '2025-01-01T00:00:00.000Z');
  assert.equal(acl.subscribe({ username: 'alice' }, 'sensors/a/state').allowed, true);
});

test('reloadAcl keeps the old rules when loading fails', async () => {
  const acl = createAcl({ rules: baseRules });

  await assert.rejects(() => reloadAcl(acl, 'C:/definitely/not/a/directory'));
  assert.equal(acl.version, 0);
  assert.equal(acl.publish({ username: 'alice' }, 'sensors/a').allowed, true);
});

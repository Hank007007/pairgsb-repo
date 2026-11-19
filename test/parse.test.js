import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  AclConfigError,
  loadAclDirectory,
  parseRuleset,
  validateRuleFilter,
} from '../src/acl/parse.js';

test('parseRuleset accepts a well formed document', () => {
  const rules = parseRuleset({
    principals: {
      'user:alice': {
        rules: [{ action: 'publish', topic: 'sensors/a/state', effect: 'allow' }],
      },
    },
  });

  assert.deepEqual(rules.principals['user:alice'].rules, [
    { action: 'publish', topic: 'sensors/a/state', effect: 'allow' },
  ]);
});

test('parseRuleset rejects malformed entries', () => {
  assert.throws(() => parseRuleset({ rules: [] }), AclConfigError);
  assert.throws(
    () =>
      parseRuleset({
        principals: {
          'user:alice': { rules: [{ action: 'publish', topic: 'a/b', effect: 'deney' }] },
        },
      }),
    AclConfigError,
  );
  assert.throws(
    () =>
      parseRuleset({
        principals: {
          'user:alice': { rules: [{ action: 'read', topic: 'a/b', effect: 'allow' }] },
        },
      }),
    AclConfigError,
  );
});

test('validateRuleFilter rejects wildcards inside a level', () => {
  assert.throws(() => validateRuleFilter('a/b+'), AclConfigError);
  assert.throws(() => validateRuleFilter('a/#/b'), AclConfigError);
  assert.throws(() => validateRuleFilter(''), AclConfigError);
});

test('loadAclDirectory merges files and ignores other extensions', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'brokeracl-'));
  try {
    await writeFile(
      join(directory, '10-base.json'),
      JSON.stringify({
        principals: {
          'user:alice': {
            rules: [{ action: 'publish', topic: 'sensors/a', effect: 'allow' }],
          },
        },
      }),
    );
    await writeFile(
      join(directory, '20-more.json'),
      JSON.stringify({
        principals: {
          'user:alice': {
            rules: [{ action: 'subscribe', topic: 'sensors/a', effect: 'allow' }],
          },
        },
      }),
    );
    await writeFile(join(directory, 'notes.txt'), 'ignored');

    const rules = await loadAclDirectory(directory);

    assert.deepEqual(
      rules.principals['user:alice'].rules.map((rule) => rule.action),
      ['publish', 'subscribe'],
    );
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test('loadAclDirectory rejects invalid JSON', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'brokeracl-'));
  try {
    await writeFile(join(directory, '10-broken.json'), '{ not json');
    await assert.rejects(() => loadAclDirectory(directory), AclConfigError);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

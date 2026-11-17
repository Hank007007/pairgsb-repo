import test from 'node:test';
import assert from 'node:assert/strict';

import { advancedSearch, parseAdvanced } from '../src/advanced.js';
import { config } from '../src/config.js';
import { createMockClient } from '../src/ldap/mock-client.js';

test('parseAdvanced fills in the configured base and scope', () => {
  const parsed = parseAdvanced({ filter: '(mail=erin.sun@corp.example)' }, config);
  assert.equal(parsed.base, 'ou=people,dc=corp,dc=example');
  assert.equal(parsed.scope, 'sub');
  assert.equal(parsed.filter, '(mail=erin.sun@corp.example)');
});

test('parseAdvanced honours explicit base, scope and attributes', () => {
  const parsed = parseAdvanced(
    {
      filter: '(objectClass=serviceAccount)',
      base: 'ou=service,dc=corp,dc=example',
      scope: 'one',
      attributes: ['cn', 'description'],
      sizeLimit: 10,
    },
    config,
  );
  assert.equal(parsed.base, 'ou=service,dc=corp,dc=example');
  assert.equal(parsed.scope, 'one');
  assert.deepEqual(parsed.attributes, ['cn', 'description']);
  assert.equal(parsed.sizeLimit, 10);
});

test('admin equality search returns the matching directory entry', async () => {
  const client = createMockClient();
  const result = await advancedSearch(
    client,
    { filter: '(title=Engineering Manager)' },
    config,
  );
  assert.equal(result.entries.length, 1);
  assert.equal(result.entries[0].attributes.uid[0], 'dave');
});

test('admin search supports a simple presence filter', async () => {
  const client = createMockClient();
  const result = await advancedSearch(
    client,
    { filter: '(objectClass=serviceAccount)', base: 'ou=service,dc=corp,dc=example', scope: 'one' },
    config,
  );
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.cn[0]).sort(),
    ['svc-reports', 'svc-sync'],
  );
});

test('admin search supports a simple conjunction', async () => {
  const client = createMockClient();
  const result = await advancedSearch(
    client,
    { filter: '(&(ou=engineering)(title=Engineer))' },
    config,
  );
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.uid[0]).sort(),
    ['bob', 'carol'],
  );
});

test('parseAdvanced rejects an empty filter', () => {
  assert.throws(() => parseAdvanced({ filter: '   ' }, config), /requires a filter/);
});

test('parseAdvanced rejects an oversized filter', () => {
  const filter = `(cn=${'a'.repeat(config.advanced.maxFilterLength)})`;
  assert.throws(() => parseAdvanced({ filter }, config), /maximum length/);
});

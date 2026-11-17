import test from 'node:test';
import assert from 'node:assert/strict';

import { config } from '../src/config.js';
import { buildEqualityFilter, buildOuFilter } from '../src/filter.js';
import { createMockClient } from '../src/ldap/mock-client.js';
import {
  normalizeOptions,
  search,
  searchOrganization,
  searchServiceAccounts,
} from '../src/search.js';

test('normalizeOptions applies the configured defaults', () => {
  const options = normalizeOptions({}, config);
  assert.equal(options.base, 'ou=people,dc=corp,dc=example');
  assert.equal(options.scope, 'sub');
  assert.equal(options.filter, '(objectClass=*)');
  assert.deepEqual(options.attributes, ['uid', 'cn', 'mail', 'ou', 'title']);
});

test('organization sync walks every page of an ou', async () => {
  const client = createMockClient();
  const result = await searchOrganization(client, 'engineering', { pageSize: 2 }, config);

  assert.equal(result.base, 'ou=people,dc=corp,dc=example');
  assert.equal(result.scope, 'sub');
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.uid[0]).sort(),
    ['alice', 'bob', 'carol', 'dave'],
  );

  const searches = client.operations.filter((operation) => operation.type === 'search');
  assert.equal(searches.length, 2);
  assert.equal(searches[1].options.pageCookie, '2');
});

test('organization sync does not return members of other ous', async () => {
  const client = createMockClient();
  const result = await searchOrganization(client, 'sales', {}, config);
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.uid[0]).sort(),
    ['erin', 'frank'],
  );
});

test('organization sync honours an explicit page size of one', async () => {
  const client = createMockClient();
  const result = await searchOrganization(client, 'sales', { pageSize: 1 }, config);
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.uid[0]),
    ['erin', 'frank'],
  );
  assert.equal(
    client.operations.filter((operation) => operation.type === 'search').length,
    2,
  );
});

test('service account sync reads one level below the service suffix', async () => {
  const client = createMockClient();
  const result = await searchServiceAccounts(client, {}, config);
  assert.equal(result.base, 'ou=service,dc=corp,dc=example');
  assert.equal(result.scope, 'one');
  assert.deepEqual(
    result.entries.map((entry) => entry.attributes.cn[0]).sort(),
    ['svc-reports', 'svc-sync'],
  );
});

test('search returns an empty list when nothing matches', async () => {
  const client = createMockClient();
  const result = await search(client, { filter: buildEqualityFilter('ou', 'legal') }, config);
  assert.deepEqual(result.entries, []);
});

test('search projects only the requested attributes', async () => {
  const client = createMockClient();
  const result = await search(
    client,
    { filter: buildEqualityFilter('uid', 'erin'), attributes: ['uid', 'mail'] },
    config,
  );
  assert.equal(result.entries.length, 1);
  assert.deepEqual(Object.keys(result.entries[0].attributes).sort(), ['mail', 'uid']);
});

test('filters built for ou values keep the attribute structure', () => {
  assert.equal(buildOuFilter('engineering'), '(ou=engineering)');
  assert.equal(buildEqualityFilter('title', 'Staff Engineer'), '(title=Staff Engineer)');
});

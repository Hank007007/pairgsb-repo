'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');

const { startServer, tokenFor, request } = require('../support/harness');

test('GET /Groups returns the tenant groups', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');

  const res = await request(ctx.base, 'GET', '/scim/v2/acme/Groups', { token });

  assert.equal(res.status, 200);
  assert.equal(res.body.totalResults, 1);
  assert.equal(res.body.Resources[0].displayName, 'Engineering');
});

test('POST /Groups creates a group with members', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');

  const res = await request(ctx.base, 'POST', '/scim/v2/acme/Groups', {
    token,
    body: {
      displayName: 'Platform',
      members: [{ value: 'acme-u0001', display: 'ada@acme.example', type: 'User' }],
    },
  });

  assert.equal(res.status, 201);
  assert.equal(res.body.displayName, 'Platform');
  assert.equal(res.body.members.length, 1);
  assert.equal(res.body.members[0].value, 'acme-u0001');
});

test('PATCH add members appends to the group', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Groups', { token });
  const group = list.body.Resources[0];

  const res = await request(ctx.base, 'PATCH', `/scim/v2/acme/Groups/${group.id}`, {
    token,
    body: {
      Operations: [
        { op: 'add', path: 'members', value: [{ value: 'acme-u0002', display: 'grace@acme.example', type: 'User' }] },
      ],
    },
  });

  assert.equal(res.status, 200);
  assert.equal(res.body.members.length, 3);
  assert.ok(res.body.members.some((m) => m.value === 'acme-u0002'));
});

test('PATCH remove a specific member leaves the others', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Groups', { token });
  const group = list.body.Resources[0];
  const memberId = group.members[0].value;

  const res = await request(ctx.base, 'PATCH', `/scim/v2/acme/Groups/${group.id}`, {
    token,
    body: {
      Operations: [
        { op: 'remove', path: `members[value eq "${memberId}"]` },
      ],
    },
  });

  assert.equal(res.status, 200);
  assert.equal(res.body.members.length, 1);
  assert.ok(!res.body.members.some((m) => m.value === memberId));
});

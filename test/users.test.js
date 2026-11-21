'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');

const { startServer, tokenFor, request, FULL_SCOPES } = require('../support/harness');

test('GET /Users returns the tenant list response', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');

  const res = await request(ctx.base, 'GET', '/scim/v2/acme/Users', { token });

  assert.equal(res.status, 200);
  assert.deepEqual(res.body.schemas, ['urn:ietf:params:scim:api:messages:2.0:ListResponse']);
  assert.equal(res.body.totalResults, 2);
  assert.equal(res.body.startIndex, 1);
  assert.equal(res.body.itemsPerPage, 2);
  assert.equal(res.body.Resources.length, 2);
  assert.ok(res.body.Resources.every((u) => u.meta.resourceType === 'User'));
});

test('GET /Users/{id} returns one user', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Users', { token });
  const id = list.body.Resources[0].id;

  const res = await request(ctx.base, 'GET', `/scim/v2/acme/Users/${id}`, { token });

  assert.equal(res.status, 200);
  assert.equal(res.body.id, id);
  assert.equal(res.body.meta.resourceType, 'User');
  assert.equal(res.headers.get('etag'), `"${res.body.meta.version}"`);
});

test('filter userName eq matches exactly one user', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');

  const res = await request(
    ctx.base,
    'GET',
    '/scim/v2/acme/Users?filter=userName%20eq%20%22ada%40acme.example%22',
    { token }
  );

  assert.equal(res.status, 200);
  assert.equal(res.body.totalResults, 1);
  assert.equal(res.body.Resources[0].userName, 'ada@acme.example');
});

test('POST /Users creates a user', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');

  const res = await request(ctx.base, 'POST', '/scim/v2/acme/Users', {
    token,
    body: {
      schemas: ['urn:ietf:params:scim:schemas:core:2.0:User'],
      userName: 'linus@acme.example',
      name: { givenName: 'Linus', familyName: 'Torvalds' },
    },
  });

  assert.equal(res.status, 201);
  assert.equal(res.body.userName, 'linus@acme.example');
  assert.equal(res.body.meta.version, '1');
  assert.ok(res.headers.get('location').endsWith(`/Users/${res.body.id}`));

  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Users', { token });
  assert.equal(list.body.totalResults, 3);
});

test('full-scope token sees sensitive attributes', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme', FULL_SCOPES);
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Users', { token });
  const ada = list.body.Resources.find((u) => u.userName === 'ada@acme.example');

  assert.ok(Array.isArray(ada.groups));
  assert.ok(Array.isArray(ada.roles));
  assert.equal(typeof ada.active, 'boolean');
});

test('PATCH replace name.givenName updates the user', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Users', { token });
  const id = list.body.Resources[0].id;

  const res = await request(ctx.base, 'PATCH', `/scim/v2/acme/Users/${id}`, {
    token,
    body: {
      schemas: ['urn:ietf:params:scim:api:messages:2.0:PatchOp'],
      Operations: [{ op: 'replace', path: 'name.givenName', value: 'Augusta' }],
    },
  });

  assert.equal(res.status, 200);
  assert.equal(res.body.name.givenName, 'Augusta');
  assert.equal(res.body.meta.version, '2');
});

test('DELETE /Groups/{id} removes the group', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());
  const token = tokenFor('acme');
  const list = await request(ctx.base, 'GET', '/scim/v2/acme/Groups', { token });
  const id = list.body.Resources[0].id;

  const res = await request(ctx.base, 'DELETE', `/scim/v2/acme/Groups/${id}`, { token });
  assert.equal(res.status, 204);

  const after = await request(ctx.base, 'GET', `/scim/v2/acme/Groups/${id}`, { token });
  assert.equal(after.status, 404);
});

test('missing token is rejected', async (t) => {
  const ctx = await startServer();
  t.after(() => ctx.close());

  const res = await request(ctx.base, 'GET', '/scim/v2/acme/Users');
  assert.equal(res.status, 401);
  assert.equal(res.body.scimType, 'invalidToken');
});

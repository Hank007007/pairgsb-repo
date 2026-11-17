import test from 'node:test';
import assert from 'node:assert/strict';

import { authenticate, findUser, userBindDN } from '../src/auth.js';
import { config } from '../src/config.js';
import { createMockClient } from '../src/ldap/mock-client.js';

test('authenticate binds with the account DN and reports the display name', async () => {
  const client = createMockClient();
  const session = await authenticate(client, { username: 'alice', password: 'alice-secret' });

  assert.equal(session.dn, 'uid=alice,ou=people,dc=corp,dc=example');
  assert.equal(session.displayName, 'Alice Zhang');
  assert.equal(session.entry.attributes.mail[0], 'alice.zhang@corp.example');
  assert.deepEqual(client.boundDns, ['uid=alice,ou=people,dc=corp,dc=example']);
});

test('authenticate rejects a wrong password without exposing account details', async () => {
  const client = createMockClient();
  await assert.rejects(
    authenticate(client, { username: 'alice', password: 'wrong-password' }),
    (error) => {
      assert.equal(error.code, 'INVALID_CREDENTIALS');
      assert.equal(error.message, 'invalid credentials');
      return true;
    },
  );
  assert.equal(client.boundDns.length, 0);
});

test('authenticate rejects an unknown account', async () => {
  const client = createMockClient();
  await assert.rejects(
    authenticate(client, { username: 'nobody', password: 'whatever' }),
    (error) => error.code === 'INVALID_CREDENTIALS' && /account not found/.test(error.message),
  );
});

test('authenticate requires both username and password', async () => {
  const client = createMockClient();
  await assert.rejects(authenticate(client, { username: '', password: 'x' }), {
    code: 'INVALID_CREDENTIALS',
  });
  await assert.rejects(authenticate(client, { username: 'alice' }), {
    code: 'INVALID_CREDENTIALS',
  });
});

test('findUser returns a single entry for a regular employee', async () => {
  const client = createMockClient();
  const entries = await findUser(client, 'bob', config);
  assert.equal(entries.length, 1);
  assert.equal(entries[0].dn, 'uid=bob,ou=people,dc=corp,dc=example');
  assert.equal(entries[0].attributes.title[0], 'Engineer');
});

test('userBindDN places the account under the people suffix', () => {
  assert.equal(userBindDN('carol', config), 'uid=carol,ou=people,dc=corp,dc=example');
});

test('the login search only asks for the configured attributes', async () => {
  const client = createMockClient();
  await findUser(client, 'dave', config);
  const [operation] = client.operations;
  assert.equal(operation.type, 'search');
  assert.deepEqual(operation.options.attributes, ['uid', 'cn', 'mail', 'ou', 'title']);
});

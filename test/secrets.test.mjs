import test from 'node:test';
import assert from 'node:assert/strict';
import { SecretStore, SecretError } from '../src/secrets.mjs';
import { createMemoryLogger } from '../src/log.mjs';

test('normalises secret names to upper case', () => {
  const secrets = new SecretStore({ token: 's3cr3t-value' });
  assert.equal(secrets.has('TOKEN'), true);
  assert.equal(secrets.get('Token'), 's3cr3t-value');
  assert.deepEqual(secrets.names(), ['TOKEN']);
});

test('reads secrets from the RUNWRIGHT_SECRET_ environment prefix', () => {
  const secrets = SecretStore.fromEnvironment({ RUNWRIGHT_SECRET_API_KEY: 'abcdef12345', OTHER: 'x' });
  assert.equal(secrets.get('API_KEY'), 'abcdef12345');
  assert.equal(secrets.has('OTHER'), false);
});

test('rejects an unknown secret name', () => {
  const secrets = new SecretStore({ TOKEN: 's3cr3t-value' });
  assert.throws(() => secrets.get('MISSING'), SecretError);
});

test('masks a secret in log text', () => {
  const secrets = new SecretStore({ TOKEN: 'correct-horse-battery' });
  const text = 'using correct-horse-battery now';
  assert.equal(secrets.mask(text), 'using *** now');
});

test('masks every occurrence of a secret', () => {
  const secrets = new SecretStore({ TOKEN: 'correct-horse-battery' });
  assert.equal(
    secrets.mask('correct-horse-battery and correct-horse-battery'),
    '*** and ***',
  );
});

test('masks secrets in logger output', () => {
  const secrets = new SecretStore({ TOKEN: 'correct-horse-battery' });
  const logger = createMemoryLogger({ secrets });
  logger.info('token=correct-horse-battery');
  logger.stepOutput('stdout', 'echo correct-horse-battery');
  assert.ok(logger.output.includes('***'));
  assert.equal(logger.output.includes('correct-horse-battery'), false);
});

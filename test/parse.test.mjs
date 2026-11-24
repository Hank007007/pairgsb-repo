import test from 'node:test';
import assert from 'node:assert/strict';

import {
  parse_policy,
  validate_source_expression,
} from '../src/policy/parse.mjs';

test('parses a single directive with one source', () => {
  assert.deepEqual(parse_policy("default-src 'self'"), {
    'default-src': ["'self'"],
  });
});

test('parses a JSON policy document', () => {
  const policy = parse_policy({
    'default-src': "'self'",
    'img-src': ['https://img.example', 'https://*.cdn.example'],
  });
  assert.deepEqual(policy, {
    'default-src': ["'self'"],
    'img-src': ['https://img.example', 'https://*.cdn.example'],
  });
});

test('accepts the source expressions the platform emits', () => {
  const accepted = [
    "'self'",
    "'unsafe-inline'",
    "'none'",
    'https:',
    'https://cdn.example',
    'https://cdn.example:8443',
    'https://*.cdn.example',
    'https://cdn.example/assets/',
    "'nonce-dGhlLXNhbXBsZQ=='",
    "'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='",
  ];
  for (const source of accepted) {
    assert.equal(
      validate_source_expression('script-src', source),
      true,
      `expected ${source} to be accepted`,
    );
  }
});

test('rejects a source expression with a non-numeric port', () => {
  assert.throws(
    () => validate_source_expression('script-src', 'https://cdn.example:abc'),
    (error) => {
      assert.equal(error.name, 'PolicyError');
      assert.equal(error.details.directive, 'script-src');
      assert.equal(error.details.source, 'https://cdn.example:abc');
      return true;
    },
  );
});

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { compileRule, clearRuleCache } = require('../src/compile');
const { evaluate } = require('../src/eval');
const { RuleError } = require('../src/errors');

test('compiles a rule and exposes the source text', () => {
  const compiled = compileRule('severity == "high"', { tenantId: 'acme' });
  assert.equal(compiled.source, 'severity == "high"');
  assert.equal(compiled.tenantId, 'acme');
  assert.ok(compiled.ast);
});

test('reuses a compiled rule for repeated events', () => {
  clearRuleCache();
  const first = compileRule('event.action == "login"', { tenantId: 'acme' });
  const second = compileRule('event.action == "login"', { tenantId: 'acme' });
  assert.equal(first, second);
  assert.equal(evaluate(first, { event: { action: 'login' } }), true);
});

test('compiles rules that use tenant-specific fields', () => {
  const compiled = compileRule('network.protocol == "tcp"', { tenantId: 'globex' });
  assert.equal(evaluate(compiled, { network: { protocol: 'tcp' } }), true);
});

test('defaults to the default tenant schema', () => {
  const compiled = compileRule('severity != "medium"');
  assert.equal(compiled.tenantId, 'default');
});

test('surfaces lexical errors with a code', () => {
  assert.throws(
    () => compileRule('severity == high + 1', { tenantId: 'acme' }),
    (error) => error instanceof RuleError && error.code === 'E_LEX_UNEXPECTED_CHAR',
  );
});

test('surfaces parse errors with a code', () => {
  assert.throws(
    () => compileRule('severity ==', { tenantId: 'acme' }),
    (error) => error instanceof RuleError && error.code === 'E_PARSE_UNEXPECTED_EOF',
  );
});

test('rejects an unknown tenant', () => {
  assert.throws(
    () => compileRule('severity == "medium"', { tenantId: 'initech' }),
    (error) => error instanceof RuleError && error.code === 'E_UNKNOWN_TENANT',
  );
});

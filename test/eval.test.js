'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const { compileRule } = require('../src/compile');
const { evaluate } = require('../src/eval');

const EVENT = {
  severity: 'high',
  message: 'login failed for alice',
  event: { action: 'login', id: 'evt-1' },
  source: { ip: '10.1.2.3', port: 443, host: 'edge-1' },
  actor: { user: 'alice', role: 'admin' },
};

function verdict(expression, event = EVENT, tenantId = 'acme') {
  const compiled = compileRule(expression, { tenantId });
  return evaluate(compiled, event);
}

test('matches a string equality against the event', () => {
  assert.equal(verdict('severity == "high"'), true);
  assert.equal(verdict('severity == "low"'), false);
});

test('reads nested field paths', () => {
  assert.equal(verdict('source.ip == "10.1.2.3"'), true);
  assert.equal(verdict('actor.user == "alice" and actor.role == "admin"'), true);
});

test('compares numbers in order', () => {
  assert.equal(verdict('source.port > 400'), true);
  assert.equal(verdict('source.port < 400'), false);
  assert.equal(verdict('source.port >= 443'), true);
});

test('combines conditions with and, or and not', () => {
  assert.equal(verdict('severity == "high" and event.action == "login"'), true);
  assert.equal(verdict('severity == "low" or event.action == "login"'), true);
  assert.equal(verdict('not severity == "low"'), true);
});

test('tests membership in an array literal', () => {
  assert.equal(verdict('event.action in ["login", "logout"]'), true);
  assert.equal(verdict('event.action in ["logout"]'), false);
});

test('tests membership in a cidr block', () => {
  assert.equal(verdict('source.ip in cidr("10.0.0.0/8")'), true);
  assert.equal(verdict('source.ip in cidr("192.168.0.0/16")'), false);
  assert.equal(verdict('severity == "high" and source.ip in cidr("10.0.0.0/8")'), true);
});

test('applies matches to the event message', () => {
  assert.equal(verdict('message matches "login failed"'), true);
  assert.equal(verdict('message matches "logout"'), false);
});

test('calls whitelisted string functions', () => {
  assert.equal(verdict('lower(severity) == "high"'), true);
  assert.equal(verdict('upper(severity) == "HIGH"'), true);
  assert.equal(verdict('contains(message, "failed")'), true);
  assert.equal(verdict('startsWith(message, "login")'), true);
  assert.equal(verdict('endsWith(message, "alice")'), true);
  assert.equal(verdict('len(actor.user) == 5'), true);
});

test('treats a field that is absent from the event as null', () => {
  assert.equal(verdict('event.time == null'), true);
  assert.equal(verdict('source.geo.country == null'), true);
});

test('compiles the same source once per tenant', () => {
  const first = compileRule('severity == "high"', { tenantId: 'acme' });
  const second = compileRule('severity == "high"', { tenantId: 'acme' });
  assert.equal(first, second);
});

test('evaluates each rule against several events independently', () => {
  const compiled = compileRule('severity == "high"', { tenantId: 'acme' });
  assert.equal(evaluate(compiled, EVENT), true);
  assert.equal(evaluate(compiled, { severity: 'low' }), false);
  assert.equal(evaluate(compiled, EVENT), true);
});

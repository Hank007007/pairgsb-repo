import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluate, truthy, toText, unwrapExpression } from '../src/expr.mjs';
import { buildGithubContext, buildBaseContexts } from '../src/context.mjs';

const github = buildGithubContext({
  event_name: 'pull_request',
  sha: 'a1b2c3d4',
  ref: 'refs/pull/42/merge',
  repository: 'acme/widgets',
  actor: 'octocat',
  pull_request: {
    number: 42,
    title: 'Add login form',
    head: { ref: 'feature/login', repo: { full_name: 'octocat/widgets' } },
    base: { ref: 'main' },
  },
  commits: [{ message: 'Fix typo' }],
});

const contexts = buildBaseContexts({
  github,
  env: { BUILD_DIR: 'artifacts', RETRIES: '3' },
  matrix: { os: 'linux', node: 20 },
  needs: { build: { result: 'success', outputs: { digest: 'abc123' } } },
});

test('reads a context path', () => {
  assert.equal(evaluate('github.repository', contexts), 'acme/widgets');
  assert.equal(evaluate('github.event.pull_request.number', contexts), 42);
  assert.equal(evaluate('github.event.commits[0].message', contexts), 'Fix typo');
  assert.equal(evaluate('matrix.node', contexts), 20);
  assert.equal(evaluate('needs.build.result', contexts), 'success');
  assert.equal(evaluate('env.BUILD_DIR', contexts), 'artifacts');
});

test('unwraps the expression markers', () => {
  assert.equal(unwrapExpression('${{ github.actor }}'), 'github.actor');
  assert.equal(evaluate('${{ github.actor }}', contexts), 'octocat');
});

test('compares values', () => {
  assert.equal(evaluate("github.event_name == 'pull_request'", contexts), true);
  assert.equal(evaluate("github.actor != 'someone-else'", contexts), true);
  assert.equal(evaluate('github.event.pull_request.number > 1', contexts), true);
  assert.equal(evaluate('github.event.pull_request.number <= 42', contexts), true);
  assert.equal(evaluate("matrix.os == 'linux' && matrix.node >= 18", contexts), true);
  assert.equal(evaluate("needs.build.result == 'failure' || false", contexts), false);
  assert.equal(evaluate('!false', contexts), true);
});

test('keeps comparison strict across types', () => {
  assert.equal(evaluate("matrix.node == '20'", contexts), false);
  assert.equal(evaluate("env.RETRIES == '3'", contexts), true);
  assert.equal(evaluate('matrix.node != 20', contexts), false);
});

test('renders scalar results as text', () => {
  assert.equal(toText(true), 'true');
  assert.equal(toText(false), 'false');
  assert.equal(toText(null), 'null');
  assert.equal(toText(20), '20');
  assert.equal(toText('text'), 'text');
});

test('applies truthiness', () => {
  assert.equal(truthy(''), false);
  assert.equal(truthy('0'), true);
  assert.equal(truthy(0), false);
  assert.equal(truthy(null), false);
  assert.equal(truthy(false), false);
  assert.equal(truthy([]), true);
  assert.equal(truthy({}), true);
});

test('rejects malformed expressions', () => {
  assert.throws(() => evaluate('github.', contexts), /malformed|unknown/i);
  assert.throws(() => evaluate('', contexts), /empty expression/i);
  assert.throws(() => evaluate('unknown_root.value', contexts), /unknown context/i);
  assert.throws(() => evaluate('github.event.pull_request.body', contexts), /not defined/i);
});

test('rejects unknown context roots', () => {
  assert.throws(() => evaluate('secrets.TOKEN', contexts), /unknown context|secrets/i);
  assert.throws(() => evaluate('vars.NAME', contexts), /unknown context/i);
});

test('short-circuits logical operators', () => {
  assert.equal(evaluate('false && github.event.pull_request.body', contexts), false);
  assert.equal(evaluate('true || github.event.pull_request.body', contexts), true);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import {
  interpolateDataValue,
  interpolateEnvMapping,
  renderScript,
  evaluateCondition,
  findExpressions,
} from '../src/interpolate.mjs';
import { buildGithubContext, buildBaseContexts } from '../src/context.mjs';
import { SecretStore } from '../src/secrets.mjs';
import { defaultShell } from '../src/runner.mjs';

const github = buildGithubContext({
  event_name: 'pull_request',
  actor: 'octocat',
  pull_request: {
    number: 42,
    title: 'Add login form',
    head: { ref: 'feature/login' },
  },
});

const contexts = buildBaseContexts({
  github,
  env: { BUILD_DIR: 'artifacts' },
  matrix: { node: 20 },
});

function runScript(script) {
  const shell = defaultShell(process.env);
  const result = spawnSync(shell.file, [...shell.args, script], {
    encoding: 'utf8',
    windowsHide: true,
  });
  return { status: result.status, stdout: result.stdout, stderr: result.stderr };
}

test('substitutes a single context value into a script', () => {
  const script = 'printf "%s\\n" ${{ github.repository }} ${{ matrix.node }}\n';
  const rendered = renderScript(script, contexts);
  const outcome = runScript(rendered);

  assert.equal(outcome.status, 0);
  assert.equal(outcome.stdout.replace(/\r\n/g, '\n'), 'local/repository\n20\n');
});

test('leaves script text outside the markers untouched', () => {
  const script = 'set -e\nnpm ci\nnpm test\n';
  assert.equal(renderScript(script, contexts), script);
});

test('substitutes values in env mappings', () => {
  const env = interpolateEnvMapping(
    { DIR: '${{ env.BUILD_DIR }}', LABEL: 'build-${{ github.event.pull_request.number }}' },
    contexts,
  );
  assert.deepEqual(env, { DIR: 'artifacts', LABEL: 'build-42' });
});

test('reads a secret when it is the whole env value', () => {
  const secrets = new SecretStore({ TOKEN: 's3cr3t-value' });
  const value = interpolateEnvMapping({ TOKEN: '${{ secrets.TOKEN }}' }, contexts, { secrets });
  assert.deepEqual(value, { TOKEN: 's3cr3t-value' });
});

test('reads a secret with literal text around it in env', () => {
  const secrets = new SecretStore({ TOKEN: 's3cr3t-value' });
  const value = interpolateEnvMapping({ AUTH: 'Bearer ${{ secrets.TOKEN }}' }, contexts, { secrets });
  assert.deepEqual(value, { AUTH: 'Bearer s3cr3t-value' });
});

test('rejects a secret referenced inside run', () => {
  const secrets = new SecretStore({ TOKEN: 's3cr3t-value' });
  assert.throws(
    () => renderScript('curl -H "Authorization: ${{ secrets.TOKEN }}"', contexts, { secrets }),
    /secrets\.TOKEN/,
  );
});

test('rejects a secret referenced inside if', () => {
  const secrets = new SecretStore({ TOKEN: 's3cr3t-value' });
  assert.throws(
    () => evaluateCondition("${{ secrets.TOKEN != '' }}", contexts, { secrets }),
    /secrets\.TOKEN/,
  );
});

test('evaluates a condition wrapper', () => {
  assert.equal(evaluateCondition("${{ github.event_name == 'pull_request' }}", contexts), true);
  assert.equal(evaluateCondition('${{ matrix.node >= 18 }}', contexts), true);
});

test('renders boolean results as text', () => {
  assert.equal(interpolateDataValue('${{ matrix.node >= 18 }}', contexts), 'true');
  assert.equal(interpolateDataValue('${{ matrix.node >= 21 }}', contexts), 'false');
});

test('finds every expression wrapper', () => {
  const found = findExpressions('a ${{ github.actor }} b ${{ matrix.node }}');
  assert.equal(found.length, 2);
  assert.equal(found[0].source, ' github.actor ');
  assert.equal(found[1].source, ' matrix.node ');
});

test('reports an unterminated wrapper', () => {
  assert.throws(() => interpolateDataValue('prefix ${{ github.actor', contexts), /unterminated/i);
});

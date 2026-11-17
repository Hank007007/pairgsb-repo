import test from 'node:test';
import assert from 'node:assert/strict';
import { parseYaml } from '../src/yaml.mjs';
import { normalizePipeline } from '../src/config.mjs';
import { runPipeline, baseEnvironment, applyEnv, defaultShell } from '../src/runner.mjs';
import { SecretStore } from '../src/secrets.mjs';
import { createMemoryLogger } from '../src/log.mjs';
import { buildGithubContext } from '../src/context.mjs';

const github = buildGithubContext({
  event_name: 'pull_request',
  repository: 'acme/widgets',
  actor: 'octocat',
});

function makePipeline(source) {
  return normalizePipeline(parseYaml(source));
}

function capture() {
  const calls = [];
  return {
    calls,
    spawnStep: async (context) => {
      calls.push(context);
      return { result: 'success', code: 0 };
    },
  };
}

test('runs every step in declaration order', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: first
        run: echo first
      - name: second
        run: echo second
`);
  const { calls, spawnStep } = capture();
  const logger = createMemoryLogger();
  const outcome = await runPipeline({
    pipeline,
    logger,
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep,
  });

  assert.equal(outcome.result, 'success');
  assert.deepEqual(calls.map((call) => call.script), ['echo first', 'echo second']);
});

test('merges pipeline, job and step environment', async () => {
  const pipeline = makePipeline(`
env:
  SHARED: pipeline
  ONLY_TOP: top
jobs:
  build:
    env:
      SHARED: job
      JOB_ONLY: job
    steps:
      - name: inspect
        env:
          STEP_ONLY: step
        run: printenv
`);
  const { calls, spawnStep } = capture();
  await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    environment: { BASE: 'base' },
    spawnStep,
  });

  const environment = calls[0].environment;
  assert.equal(environment.BASE, 'base');
  assert.equal(environment.ONLY_TOP, 'top');
  assert.equal(environment.SHARED, 'job');
  assert.equal(environment.JOB_ONLY, 'job');
  assert.equal(environment.STEP_ONLY, 'step');
});

test('passes with: entries of a run step as environment', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: build
        run: echo building
        with:
          TARGET: release
`);
  const { calls, spawnStep } = capture();
  await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    environment: {},
    spawnStep,
  });
  assert.equal(calls[0].environment.TARGET, 'release');
});

test('skips a step whose condition is false', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: never
        if: \${{ github.event_name == 'push' }}
        run: echo no
      - name: always
        if: \${{ github.event_name == 'pull_request' }}
        run: echo yes
`);
  const { calls, spawnStep } = capture();
  const outcome = await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep,
  });

  assert.equal(outcome.result, 'success');
  assert.equal(outcome.jobs.build.steps[0].result, 'skipped');
  assert.deepEqual(calls.map((call) => call.script), ['echo yes']);
});

test('stops the job after a failing step', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: fail
        run: exit 1
      - name: unreachable
        run: echo later
`);
  const calls = [];
  const outcome = await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep: async (context) => {
      calls.push(context.script);
      return { result: 'failure', code: 1 };
    },
  });

  assert.equal(outcome.result, 'failure');
  assert.deepEqual(calls, ['exit 1']);
});

test('treats a non-boolean continue-on-error as false', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: flaky
        continue-on-error: truly
        run: exit 1
      - name: next
        run: echo next
`);
  const calls = [];
  const outcome = await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep: async (context) => {
      calls.push(context.script);
      return { result: 'failure', code: 1 };
    },
  });
  assert.equal(outcome.result, 'failure');
});

test('continues after a failure marked continue-on-error: true', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: flaky
        continue-on-error: true
        run: exit 1
      - name: next
        run: echo next
`);
  const calls = [];
  const outcome = await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep: async (context) => {
      calls.push(context.script);
      return context.script === 'exit 1'
        ? { result: 'failure', code: 1 }
        : { result: 'success', code: 0 };
    },
  });
  assert.equal(outcome.result, 'success');
  assert.equal(calls.length, 2);
});

test('exposes needs results to dependent jobs', async () => {
  const pipeline = makePipeline(`
jobs:
  first:
    steps:
      - name: one
        run: echo one
  second:
    needs: [first]
    steps:
      - name: two
        if: \${{ needs.first.result == 'success' }}
        run: echo two
`);
  const { calls, spawnStep } = capture();
  const outcome = await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    spawnStep,
  });
  assert.equal(outcome.result, 'success');
  assert.equal(calls.length, 2);
});

test('rejects a job whose dependency was not run first', async () => {
  const pipeline = makePipeline(`
jobs:
  second:
    needs: [first]
    steps:
      - name: two
        run: echo two
  first:
    steps:
      - name: one
        run: echo one
`);
  await assert.rejects(
    runPipeline({
      pipeline,
      logger: createMemoryLogger(),
      secrets: new SecretStore(),
      github,
      githubEvent: {},
      spawnStep: async () => ({ result: 'success', code: 0 }),
    }),
    /needs unknown or unordered/,
  );
});

test('resolves a local action relative to the workspace', async () => {
  const workspace = new URL('./fixtures/workspace/', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1');
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: packaged
        uses: local:scripts/hello.sh
`);
  const { calls, spawnStep } = capture();
  await runPipeline({
    pipeline,
    logger: createMemoryLogger(),
    secrets: new SecretStore(),
    github,
    githubEvent: { event_name: 'pull_request' },
    workspace,
    spawnStep,
  });
  assert.match(calls[0].script, /hello from an action/);
});

test('refuses a local action that escapes the workspace', async () => {
  const pipeline = makePipeline(`
jobs:
  build:
    steps:
      - name: escaped
        uses: local:../outside.sh
`);
  await assert.rejects(
    runPipeline({
      pipeline,
      logger: createMemoryLogger(),
      secrets: new SecretStore(),
      github,
      githubEvent: {},
      workspace: new URL('./fixtures/workspace/', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1'),
      spawnStep: async () => ({ result: 'success', code: 0 }),
    }),
    /escapes the workspace/,
  );
});

test('builds a child environment without the runner secret variables', () => {
  const environment = baseEnvironment({ PATH: '/usr/bin', RUNWRIGHT_SECRET_TOKEN: 'abc' });
  assert.equal(environment.PATH, '/usr/bin');
  assert.equal('RUNWRIGHT_SECRET_TOKEN' in environment, false);
});

test('applyEnv adds entries to the target', () => {
  const target = { A: '1' };
  applyEnv(target, { B: '2' });
  assert.deepEqual(target, { A: '1', B: '2' });
});

test('selects a shell for the current platform', () => {
  const shell = defaultShell({ SHELL: '' });
  assert.equal(typeof shell.file, 'string');
  assert.ok(Array.isArray(shell.args));
});

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { parseYaml } from '../src/yaml.mjs';
import { normalizePipeline } from '../src/config.mjs';
import { runPipeline } from '../src/runner.mjs';
import { SecretStore } from '../src/secrets.mjs';
import { createMemoryLogger } from '../src/log.mjs';
import { buildGithubContext } from '../src/context.mjs';

const github = buildGithubContext({ event_name: 'pull_request', repository: 'acme/widgets' });

function workspace() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'runwright-run-'));
}

test('executes a real shell step and captures its output', async () => {
  const cwd = workspace();
  const pipeline = normalizePipeline(parseYaml(`
jobs:
  build:
    steps:
      - name: hello
        run: echo "hello from a step"
`));
  const logger = createMemoryLogger();

  const outcome = await runPipeline({
    pipeline,
    workspace: cwd,
    environment: { PATH: process.env.PATH, SHELL: process.env.SHELL ?? '' },
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    logger,
  });

  assert.equal(outcome.result, 'success');
  assert.match(logger.stdout, /hello from a step/);
});

test('propagates a non-zero exit status as a failed job', async () => {
  const pipeline = normalizePipeline(parseYaml(`
jobs:
  build:
    steps:
      - name: fail
        run: exit 3
`));
  const outcome = await runPipeline({
    pipeline,
    workspace: workspace(),
    environment: { PATH: process.env.PATH, SHELL: process.env.SHELL ?? '' },
    secrets: new SecretStore(),
    github,
    githubEvent: {},
    logger: createMemoryLogger(),
  });
  assert.equal(outcome.result, 'failure');
  assert.equal(outcome.jobs.build.steps[0].code, 3);
});

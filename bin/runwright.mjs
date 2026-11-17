#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { parseYaml } from '../src/yaml.mjs';
import { normalizePipeline, ConfigError } from '../src/config.mjs';
import { runPipeline } from '../src/runner.mjs';
import { SecretStore, SecretError } from '../src/secrets.mjs';
import { createLogger } from '../src/log.mjs';
import { buildGithubContext } from '../src/context.mjs';
import { ExpressionError } from '../src/expr.mjs';

function readJson(file) {
  if (!file) return {};
  const text = fs.readFileSync(file, 'utf8');
  return text.trim() === '' ? {} : JSON.parse(text);
}

async function main(argv) {
  const { values } = parseArgs({
    args: argv,
    options: {
      file: { type: 'string', default: 'pipeline.yml' },
      workspace: { type: 'string', default: process.cwd() },
      event: { type: 'string' },
      matrix: { type: 'string' },
      'github-repository': { type: 'string' },
      'event-name': { type: 'string' },
      timeout: { type: 'string' },
      quiet: { type: 'boolean', default: false },
    },
    allowPositionals: false,
  });

  const workspace = path.resolve(values.workspace);
  const file = path.resolve(workspace, values.file);

  if (!fs.existsSync(file)) {
    process.stderr.write(`error: pipeline file not found: ${values.file}\n`);
    return 2;
  }

  const secrets = SecretStore.fromEnvironment(process.env);
  const logger = createLogger({ secrets });

  let pipeline;
  try {
    pipeline = normalizePipeline(parseYaml(fs.readFileSync(file, 'utf8')));
  } catch (error) {
    logger.error(`invalid pipeline: ${error.message}`);
    return 2;
  }

  const event = values.event ? readJson(path.resolve(workspace, values.event)) : {};
  const matrix = values.matrix ? readJson(path.resolve(workspace, values.matrix)) : {};
  const github = buildGithubContext(event, {
    repository: values['github-repository'],
    eventName: values['event-name'],
  });

  const timeout = values.timeout ? Number(values.timeout) : undefined;

  logger.info(`runwright ${pipeline.name}`);

  try {
    const outcome = await runPipeline({
      pipeline,
      workspace,
      secrets,
      logger: values.quiet ? { ...logger, info() {} } : logger,
      matrix,
      github,
      githubEvent: event,
      timeout,
    });
    logger.info(`result: ${outcome.result}`);
    return outcome.result === 'success' ? 0 : 1;
  } catch (error) {
    if (error instanceof ConfigError || error instanceof ExpressionError || error instanceof SecretError) {
      logger.error(error.message);
      return 2;
    }
    logger.error(error && error.stack ? error.stack : String(error));
    return 1;
  }
}

main(process.argv.slice(2)).then(
  (code) => {
    process.exitCode = code;
  },
  (error) => {
    process.stderr.write(`error: ${error && error.stack ? error.stack : String(error)}\n`);
    process.exitCode = 1;
  },
);

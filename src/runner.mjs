import { spawn } from 'node:child_process';
import path from 'node:path';
import fs from 'node:fs';
import { createLogger } from './log.mjs';
import { SecretStore } from './secrets.mjs';
import { ConfigError } from './config.mjs';
import { prepareSteps, resolveLocalAction, summarizeNeeds, buildGithubContext } from './context.mjs';
import { renderScript } from './interpolate.mjs';

const DEFAULT_TIMEOUT = 10 * 60 * 1000;

export function applyEnv(target, additions) {
  for (const [key, value] of Object.entries(additions)) {
    target[key] = value;
  }
  return target;
}

export function baseEnvironment(source = process.env) {
  const result = {};
  for (const [key, value] of Object.entries(source)) {
    if (key.startsWith('RUNWRIGHT_SECRET_')) continue;
    if (value === undefined) continue;
    result[key] = value;
  }
  return result;
}

export function defaultShell(environment = process.env) {
  if (process.platform === 'win32') {
    const shellPath = environment.SHELL ?? '';
    if (/bash(\.exe)?$/i.test(shellPath)) {
      return { file: shellPath, args: ['-e', '-c'], label: 'bash' };
    }
    const posix = findOnPath('bash.exe', environment) ?? findOnPath('sh.exe', environment);
    if (posix) {
      return { file: posix, args: ['-e', '-c'], label: 'bash' };
    }
    return { file: environment.ComSpec ?? 'cmd.exe', args: ['/d', '/s', '/c'], label: 'cmd' };
  }
  return { file: 'sh', args: ['-e', '-c'], label: 'sh' };
}

function findOnPath(executable, environment) {
  const pathValue = environment.PATH ?? environment.Path ?? '';
  for (const entry of pathValue.split(path.delimiter)) {
    if (entry === '') continue;
    const candidate = path.join(entry, executable);
    try {
      if (fs.existsSync(candidate) && fs.statSync(candidate).isFile()) return candidate;
    } catch {
      // A directory on PATH that cannot be read is simply skipped.
    }
  }
  return null;
}

export async function runPipeline(options) {
  const {
    pipeline,
    workspace = process.cwd(),
    environment = process.env,
    secrets = SecretStore.fromEnvironment(),
    logger = createLogger({ secrets }),
    matrix = {},
    githubEvent = {},
    githubOptions = {},
    shell = defaultShell(environment),
    timeout = DEFAULT_TIMEOUT,
    spawnStep,
    onStep,
  } = options;

  const github = options.github ?? buildGithubContext(githubEvent, githubOptions);
  const results = {};
  const completed = {};

  for (const job of pipeline.jobs) {
    const missing = job.needs.filter((id) => !completed[id]);
    if (missing.length > 0) {
      throw new ConfigError(`job "${job.id}" needs unknown or unordered job(s): ${missing.join(', ')}`);
    }

    const needsContext = summarizeNeeds(
      Object.fromEntries(job.needs.map((id) => [id, completed[id]])),
    );

    const prepared = prepareSteps(job, {
      contexts: { github, matrix, needs: needsContext, env: {} },
      baseEnv: baseEnvironment(environment),
      pipelineEnv: pipeline.env,
      secrets,
      renderScript,
    });

    const jobResult = {
      id: job.id,
      result: 'success',
      steps: [],
      outputs: {},
    };

    for (const step of prepared.steps) {
      if (!step.include) {
        jobResult.steps.push({ name: step.name, result: 'skipped' });
        logger.info(`[${job.id}] ${step.name} — skipped`);
        if (onStep) onStep({ job: job.id, step: step.name, result: 'skipped' });
        continue;
      }

      logger.info(`[${job.id}] ${step.name}`);

      const childEnv = applyEnv({ ...step.processEnv }, step.resolvedEnv);

      const outcome = await executeStep({
        job,
        workspace,
        step,
        environment: childEnv,
        shell,
        timeout: job.timeout ?? timeout,
        logger,
        spawnStep,
      });

      jobResult.steps.push({ name: step.name, result: outcome.result, code: outcome.code });

      if (onStep) {
        onStep({ job: job.id, step: step.name, result: outcome.result, code: outcome.code });
      }

      if (outcome.result === 'failure' && !step.continueOnError) {
        jobResult.result = 'failure';
        break;
      }
    }

    completed[job.id] = jobResult;
    results[job.id] = jobResult;
  }

  const failed = Object.values(results).some((job) => job.result === 'failure');
  return { result: failed ? 'failure' : 'success', jobs: results };
}

async function executeStep(context) {
  const { workspace, step, environment, shell, timeout, logger, spawnStep } = context;

  let script = step.run;
  if (script === null) {
    const actionPath = resolveLocalAction(step.uses, workspace);
    if (!fs.existsSync(actionPath) || !fs.statSync(actionPath).isFile()) {
      logger.error(`local action not found: ${step.uses}`);
      return { result: 'failure', code: 127 };
    }
    script = fs.readFileSync(actionPath, 'utf8');
  }

  if (spawnStep) {
    return spawnStep({ script, environment, workspace, shell, timeout, logger });
  }

  return new Promise((resolve) => {
    let settled = false;
    const child = spawn(shell.file, [...shell.args, script], {
      cwd: path.resolve(workspace),
      env: environment,
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
    });

    const timer = timeout > 0
      ? setTimeout(() => {
        if (settled) return;
        logger.error(`step exceeded ${timeout}ms and was terminated`);
        child.kill('SIGKILL');
      }, timeout)
      : null;
    if (timer && typeof timer.unref === 'function') timer.unref();

    forwardLines(child.stdout, (line) => logger.stepOutput('stdout', line));
    forwardLines(child.stderr, (line) => logger.stepOutput('stderr', line));

    const finish = (result, code) => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      resolve({ result, code });
    };

    child.on('error', (error) => {
      logger.error(`could not start step: ${error.message}`);
      finish('failure', 127);
    });
    child.on('close', (code) => {
      finish(code === 0 ? 'success' : 'failure', code ?? 1);
    });
  });
}

function forwardLines(stream, write) {
  let buffer = '';
  stream.setEncoding('utf8');
  stream.on('data', (chunk) => {
    buffer += chunk;
    let newline = buffer.indexOf('\n');
    while (newline !== -1) {
      write(buffer.slice(0, newline).replace(/\r$/, ''));
      buffer = buffer.slice(newline + 1);
      newline = buffer.indexOf('\n');
    }
  });
  stream.on('end', () => {
    if (buffer !== '') write(buffer.replace(/\r$/, ''));
    buffer = '';
  });
}

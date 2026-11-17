import path from 'node:path';
import {
  interpolateDataValue,
  interpolateEnvMapping,
  evaluateCondition,
} from './interpolate.mjs';
import { ExpressionError, toText } from './expr.mjs';
import { ConfigError } from './config.mjs';

export function buildGithubContext(event = {}, options = {}) {
  const repository = options.repository ?? event.repository ?? 'local/repository';
  const pullRequest = event.pull_request ?? null;

  const context = {
    event_name: options.eventName ?? event.event_name ?? 'workflow_dispatch',
    sha: event.sha ?? options.sha ?? '',
    ref: event.ref ?? options.ref ?? '',
    repository,
    actor: event.actor ?? options.actor ?? 'local',
    head_ref: pullRequest ? pullRequest.head?.ref ?? '' : '',
    event: {
      ...event,
    },
  };

  if (event.repository && typeof event.repository === 'object') {
    context.repository = event.repository.full_name ?? repository;
  }
  if (!context.event.repository) context.event.repository = context.repository;

  return deepFreeze(context);
}

export function buildBaseContexts({ github, env = {}, matrix = {}, needs = {} } = {}) {
  return {
    github: github ?? buildGithubContext({}),
    env: { ...env },
    matrix: { ...matrix },
    needs: { ...needs },
  };
}

export function prepareSteps(job, state) {
  const baseEnv = { ...state.baseEnv };
  const rootContexts = {
    github: state.contexts.github,
    env: baseEnv,
    matrix: { ...state.contexts.matrix, ...(job.matrix ?? {}) },
    needs: { ...state.contexts.needs },
  };

  const pipelineEnv = interpolateEnvMapping(state.pipelineEnv, rootContexts, {
    secrets: state.secrets,
    field: 'env',
  });
  const jobContexts = { ...rootContexts, env: { ...baseEnv, ...pipelineEnv } };

  const jobEnv = interpolateEnvMapping(job.env, jobContexts, {
    secrets: state.secrets,
    field: `jobs.${job.id}.env`,
  });
  const jobScopeEnv = { ...baseEnv, ...pipelineEnv, ...jobEnv };

  const steps = [];

  for (const step of job.steps) {
    const stepContexts = { ...rootContexts, env: { ...jobScopeEnv } };

    let include = true;
    if (step.if !== null) {
      include = evaluateCondition(step.if, stepContexts, {
        secrets: state.secrets,
        field: `jobs.${job.id}.steps[${step.index}].if`,
      });
    }

    if (!include) {
      steps.push({
        ...step,
        include: false,
        run: null,
        uses: null,
        resolvedEnv: {},
        processEnv: { ...jobScopeEnv },
      });
      continue;
    }

    const stepEnv = interpolateEnvMapping(step.env, stepContexts, {
      secrets: state.secrets,
      field: `jobs.${job.id}.steps[${step.index}].env`,
    });
    const stepWith = interpolateEnvMapping(step.with, stepContexts, {
      secrets: state.secrets,
      field: `jobs.${job.id}.steps[${step.index}].with`,
    });

    let resolvedRun = null;
    let resolvedUses = null;

    if (step.run !== null) {
      resolvedRun = state.renderScript(step.run, stepContexts, {
        secrets: state.secrets,
        field: `jobs.${job.id}.steps[${step.index}].run`,
      });
    } else {
      resolvedUses = interpolateDataValue(step.uses, stepContexts, {
        secrets: state.secrets,
        field: `jobs.${job.id}.steps[${step.index}].uses`,
      });
    }

    const name = interpolateDataValue(step.name, stepContexts, {
      secrets: state.secrets,
      field: `jobs.${job.id}.steps[${step.index}].name`,
    });

    steps.push({
      ...step,
      name,
      include: true,
      run: resolvedRun,
      uses: resolvedUses,
      resolvedEnv: { ...stepEnv, ...stepWith },
      processEnv: { ...jobScopeEnv },
    });
  }

  return {
    id: job.id,
    needs: job.needs,
    timeout: job.timeout,
    steps,
  };
}

export function resolveLocalAction(uses, workspace) {
  if (!String(uses).startsWith('local:')) {
    throw new ConfigError(`unsupported action "${uses}" (only "local:<path>" is available)`);
  }
  const relative = String(uses).slice('local:'.length);
  if (relative === '') {
    throw new ConfigError('local action path is empty');
  }
  const root = path.resolve(workspace);
  const target = path.resolve(root, relative);
  const relation = path.relative(root, target);
  if (relation === '' || relation.startsWith('..') || path.isAbsolute(relation)) {
    throw new ConfigError(`local action "${relative}" escapes the workspace`);
  }
  return target;
}

export function summarizeNeeds(needs) {
  const summary = {};
  for (const [id, result] of Object.entries(needs)) {
    summary[id] = { result: result.result ?? 'success', outputs: { ...(result.outputs ?? {}) } };
  }
  return summary;
}

export function renderContextValue(value) {
  return toText(value);
}

function deepFreeze(value) {
  if (value === null || typeof value !== 'object') return value;
  Object.freeze(value);
  for (const entry of Object.values(value)) deepFreeze(entry);
  return value;
}

export { ExpressionError, ConfigError };

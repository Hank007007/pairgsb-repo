export class ConfigError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ConfigError';
  }
}

const RESERVED_ENV_NAMES = new Set([
  'NODE_OPTIONS',
  'NODE_PATH',
  'NODE_EXTRA_CA_CERTS',
  'PATH',
  'LD_PRELOAD',
  'LD_LIBRARY_PATH',
  'DYLD_INSERT_LIBRARIES',
  'SHELL',
  'COMSPEC',
  'PATHEXT',
]);

export function isReservedEnvName(name) {
  const upper = String(name).toUpperCase();
  return RESERVED_ENV_NAMES.has(upper) || upper.startsWith('RUNWRIGHT_');
}

function assertEnvKeysAllowed(mapping, field) {
  for (const key of Object.keys(mapping)) {
    if (isReservedEnvName(key)) {
      throw new ConfigError(`"${field}.${key}" names a reserved environment variable`);
    }
  }
}

export function normalizePipeline(document) {
  if (document === null || typeof document !== 'object' || Array.isArray(document)) {
    throw new ConfigError('pipeline must be a mapping');
  }

  const name = optionalString(document.name, 'name') ?? 'pipeline';
  const env = normalizeEnv(document.env, 'env');
  assertEnvKeysAllowed(env, 'env');

  if (document.jobs === null || typeof document.jobs !== 'object' || Array.isArray(document.jobs)) {
    throw new ConfigError('pipeline must declare a "jobs" mapping');
  }

  const jobEntries = Object.entries(document.jobs);
  if (jobEntries.length === 0) {
    throw new ConfigError('"jobs" must contain at least one job');
  }

  const jobs = jobEntries.map(([id, rawJob]) => normalizeJob(id, rawJob));

  return { name, env, jobs };
}

function normalizeJob(id, rawJob) {
  if (rawJob === null || typeof rawJob !== 'object' || Array.isArray(rawJob)) {
    throw new ConfigError(`job "${id}" must be a mapping`);
  }

  const env = normalizeEnv(rawJob.env, `jobs.${id}.env`);
  assertEnvKeysAllowed(env, `jobs.${id}.env`);
  const matrix = normalizeEnv(rawJob.matrix, `jobs.${id}.matrix`);
  const needs = normalizeNeeds(rawJob.needs, `jobs.${id}.needs`);

  if (!Array.isArray(rawJob.steps) || rawJob.steps.length === 0) {
    throw new ConfigError(`job "${id}" must declare a non-empty "steps" sequence`);
  }

  const steps = rawJob.steps.map((rawStep, index) => normalizeStep(id, index, rawStep));

  const timeout = rawJob.timeout;
  if (timeout !== undefined && timeout !== null && typeof timeout !== 'number') {
    throw new ConfigError(`job "${id}".timeout must be a number of milliseconds`);
  }

  return { id, env, matrix, needs, steps, timeout: timeout ?? null };
}

function normalizeNeeds(value, field) {
  if (value === undefined || value === null) return [];
  if (typeof value === 'string') return [value];
  if (!Array.isArray(value) || value.some((entry) => typeof entry !== 'string')) {
    throw new ConfigError(`"${field}" must be a sequence of job ids`);
  }
  return value;
}

function normalizeStep(jobId, index, rawStep) {
  if (rawStep === null || typeof rawStep !== 'object' || Array.isArray(rawStep)) {
    throw new ConfigError(`job "${jobId}" step ${index} must be a mapping`);
  }

  const label = `jobs.${jobId}.steps[${index}]`;
  const hasRun = rawStep.run !== undefined;
  const hasUses = rawStep.uses !== undefined;

  if (hasRun === hasUses) {
    throw new ConfigError(`step ${index} in job "${jobId}" must declare exactly one of "run" or "uses"`);
  }
  if (hasRun && typeof rawStep.run !== 'string') {
    throw new ConfigError(`"${label}.run" must be a string`);
  }
  if (hasUses && typeof rawStep.uses !== 'string') {
    throw new ConfigError(`"${label}.uses" must be a string`);
  }

  const name = optionalString(rawStep.name, `${label}.name`);

  let condition = null;
  if (rawStep.if !== undefined && rawStep.if !== null) {
    if (typeof rawStep.if !== 'string') {
      throw new ConfigError(`"${label}.if" must be a string`);
    }
    condition = rawStep.if;
  }

  const stepEnv = normalizeEnv(rawStep.env, `${label}.env`);
  assertEnvKeysAllowed(stepEnv, `${label}.env`);
  const stepWith = normalizeEnv(rawStep.with, `${label}.with`);
  if (hasRun) {
    assertEnvKeysAllowed(stepWith, `${label}.with`);
  }

  return {
    index,
    name: name ?? `step ${index + 1}`,
    run: hasRun ? rawStep.run : null,
    uses: hasUses ? rawStep.uses : null,
    with: stepWith,
    env: stepEnv,
    if: condition,
    continueOnError: rawStep['continue-on-error'] === true,
  };
}

function normalizeEnv(value, field) {
  if (value === undefined || value === null) return {};
  if (typeof value !== 'object' || Array.isArray(value)) {
    throw new ConfigError(`"${field}" must be a mapping`);
  }
  const result = {};
  for (const [key, entry] of Object.entries(value)) {
    if (entry === null || entry === undefined || typeof entry === 'object') {
      throw new ConfigError(`"${field}.${key}" must be a scalar`);
    }
    result[String(key)] = String(entry);
  }
  return result;
}

function optionalString(value, field) {
  if (value === undefined || value === null) return null;
  if (typeof value !== 'string') {
    throw new ConfigError(`"${field}" must be a string`);
  }
  return value;
}

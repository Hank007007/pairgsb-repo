import { readFileSync } from 'node:fs';
import { CODES, makeError } from './errors.js';
import { envOverlay } from './env.js';
import { deepMerge } from './merge.js';
import { readPath, setPath, splitDottedPath } from './paths.js';
import { deepClone, isPlainObject } from './util.js';

function loadLayerFile(filePath, errors) {
  let text;
  try {
    text = readFileSync(filePath, 'utf8');
  } catch (cause) {
    errors.push(makeError(CODES.TYPE, filePath, `cannot read layer: ${cause.message}`));
    return {};
  }
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch (cause) {
    errors.push(makeError(CODES.TYPE, filePath, `layer is not valid JSON: ${cause.message}`));
    return {};
  }
  if (!isPlainObject(parsed)) {
    errors.push(makeError(CODES.TYPE, filePath, 'layer must contain a JSON object'));
    return {};
  }
  return parsed;
}

function coerceScalar(raw, existing, found) {
  if (!found) return { ok: true, value: raw };
  if (typeof existing === 'boolean') {
    const lowered = raw.trim().toLowerCase();
    if (lowered === 'true') return { ok: true, value: true };
    if (lowered === 'false') return { ok: true, value: false };
    return { ok: false };
  }
  if (typeof existing === 'number') {
    if (raw.trim() === '') return { ok: false };
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return { ok: false };
    return { ok: true, value: parsed };
  }
  if (Array.isArray(existing)) {
    const elementType = existing.length > 0 ? typeof existing[0] : 'string';
    if (elementType === 'number') {
      const parsed = Number(raw);
      if (!Number.isFinite(parsed)) return { ok: false };
      return { ok: true, value: parsed, append: true };
    }
    if (elementType === 'boolean') {
      const lowered = raw.trim().toLowerCase();
      if (lowered === 'true') return { ok: true, value: true, append: true };
      if (lowered === 'false') return { ok: true, value: false, append: true };
      return { ok: false };
    }
    return { ok: true, value: raw, append: true };
  }
  return { ok: true, value: raw };
}

function applySetFlags(config, argv, errors) {
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument !== '--set' && !argument.startsWith('--set=')) continue;

    let assignment;
    if (argument === '--set') {
      assignment = argv[index + 1];
      index += 1;
      if (assignment === undefined) {
        errors.push(makeError(CODES.INVALID_KEY, '', '--set requires path=value'));
        continue;
      }
    } else {
      assignment = argument.slice('--set='.length);
    }

    const separator = assignment.indexOf('=');
    if (separator < 0) {
      errors.push(makeError(CODES.INVALID_KEY, assignment, '--set requires path=value'));
      continue;
    }
    const pathText = assignment.slice(0, separator);
    const raw = assignment.slice(separator + 1);
    const segments = splitDottedPath(pathText);
    if (pathText.length === 0 || segments.some((segment) => segment.length === 0)) {
      errors.push(makeError(CODES.INVALID_KEY, pathText, 'path has an empty segment'));
      continue;
    }

    const existing = readPath(config, segments);
    const coerced = coerceScalar(raw, existing.value, existing.found);
    if (!coerced.ok) {
      errors.push(makeError(CODES.TYPE, pathText, 'value does not match the existing type'));
      continue;
    }
    if (coerced.append && Array.isArray(existing.value)) {
      existing.value.push(coerced.value);
    } else {
      setPath(config, pathText, coerced.value);
    }
  }
}

/**
 * Compose the configuration document from every layer.
 *
 * @param {object} options
 * @param {object} [options.defaults] lowest-precedence values
 * @param {string[]} [options.layers] paths to JSON files; the first is the base
 * @param {object} [options.env] environment variables (defaults to `process.env`)
 * @param {string} [options.prefix] environment variable prefix
 * @param {string[]} [options.argv] command line arguments (defaults to `process.argv.slice(2)`)
 * @returns {{ config: object, errors: Array<{code: string, path: string, message: string}> }}
 */
export function load(options = {}) {
  const {
    defaults = {},
    layers = [],
    env = process.env,
    prefix = 'APP_',
    argv = process.argv.slice(2),
  } = options;

  const errors = [];
  const config = deepClone(defaults);

  if (layers.length > 0) {
    deepMerge(config, loadLayerFile(layers[0], errors));
    for (let index = 1; index < layers.length; index += 1) {
      deepMerge(config, loadLayerFile(layers[index], errors));
    }
  }

  envOverlay(config, env, { prefix });
  applySetFlags(config, argv, errors);

  return { config, errors };
}

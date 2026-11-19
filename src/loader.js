import { readFileSync } from 'node:fs';
import { coerceText } from './coerce.js';
import { CODES, makeError } from './errors.js';
import { envOverlay } from './env.js';
import { deepMerge } from './merge.js';
import { readPath, setPath, splitDottedPath } from './paths.js';
import {
  classifyKey,
  isPlainObject,
  sanitizeJson,
} from './util.js';

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

    let rejected = false;
    for (const segment of segments) {
      const classification = classifyKey(segment);
      if (!classification.ok) {
        errors.push(makeError(
          classification.code,
          pathText,
          `key ${JSON.stringify(segment)} is not allowed`,
        ));
        rejected = true;
        break;
      }
    }
    if (rejected) continue;

    const existing = readPath(config, segments);
    const coerced = coerceText(raw, existing.value, existing.found);
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

function initialDocument(defaults, errors) {
  if (!isPlainObject(defaults)) {
    errors.push(makeError(CODES.TYPE, '', 'defaults must be a plain object'));
    return {};
  }
  const clean = sanitizeJson(defaults, { errors, path: '', style: 'dotted' });
  return clean.ok ? clean.value : {};
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
  const config = initialDocument(defaults, errors);

  if (layers.length > 0) {
    deepMerge(config, loadLayerFile(layers[0], errors), {
      errors,
      path: '',
      style: 'dotted',
    });
    for (let index = 1; index < layers.length; index += 1) {
      deepMerge(config, loadLayerFile(layers[index], errors), {
        errors,
        path: '',
        style: 'dotted',
      });
    }
  }

  envOverlay(config, env, { prefix, errors });
  applySetFlags(config, argv, errors);

  return { config, errors };
}

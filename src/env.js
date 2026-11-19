import { coerceText } from './coerce.js';
import { CODES, makeError } from './errors.js';
import { classifyKey, defineValue, isPlainObject } from './util.js';

const hasOwn = (target, key) => Object.prototype.hasOwnProperty.call(target, key);

/**
 * Apply environment variables carrying `prefix` on top of `config`.
 *
 * `APP_DB__POOL__SIZE=8` addresses the path `db.pool.size`; a single
 * underscore stays part of the key name. Segments are lower-cased because
 * environment variables are conventionally written in upper case.
 */
export function envOverlay(config, env = {}, options = {}, externalErrors) {
  if (config === null || typeof config !== 'object') return config;
  if (env === null || typeof env !== 'object') return config;

  const settings = options || {};
  const errors = externalErrors || settings.errors || [];
  const { prefix = 'APP_' } = settings;

  for (const name of Object.keys(env)) {
    if (!name.startsWith(prefix)) continue;

    const remainder = name.slice(prefix.length);
    const segments = remainder
      .split('__')
      .map((segment) => segment.toLowerCase());
    const pathText = segments.join('.');

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
      if (segment.includes('.')) {
        errors.push(makeError(
          CODES.INVALID_KEY,
          pathText,
          `key ${JSON.stringify(segment)} contains the path separator`,
        ));
        rejected = true;
        break;
      }
    }
    if (rejected) continue;

    const raw = env[name];
    const rawType = typeof raw;
    if (raw !== null && rawType !== 'string' && rawType !== 'number' && rawType !== 'boolean') {
      errors.push(makeError(
        CODES.TYPE,
        pathText,
        'environment value is not a JSON value',
      ));
      continue;
    }

    let cursor = config;
    for (let index = 0; index < segments.length - 1; index += 1) {
      const key = segments[index];
      const next = hasOwn(cursor, key) ? cursor[key] : undefined;
      if (!isPlainObject(next) && !Array.isArray(next)) {
        defineValue(cursor, key, {});
      }
      cursor = cursor[key];
    }

    const key = segments[segments.length - 1];
    const found = hasOwn(cursor, key);
    const existing = found ? cursor[key] : undefined;
    const coerced = coerceText(raw, existing, found);
    if (!coerced.ok) {
      errors.push(makeError(
        CODES.TYPE,
        pathText,
        'value does not match the existing type',
      ));
      continue;
    }

    if (coerced.append && Array.isArray(existing)) {
      existing.push(coerced.value);
    } else {
      defineValue(cursor, key, coerced.value);
    }
  }

  return config;
}

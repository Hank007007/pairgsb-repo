function isContainer(value) {
  return value !== null && (typeof value === 'object' || typeof value === 'function');
}

/**
 * Apply environment variables carrying `prefix` on top of `config`.
 *
 * `APP_DB__POOL__SIZE=8` addresses the path `db.pool.size`; a single
 * underscore stays part of the key name. Segments are lower-cased because
 * environment variables are conventionally written in upper case.
 */
export function envOverlay(config, env = {}, options = {}) {
  const { prefix = 'APP_' } = options;

  for (const name of Object.keys(env)) {
    if (!name.startsWith(prefix)) continue;

    const remainder = name.slice(prefix.length);
    const segments = remainder
      .split('__')
      .map((segment) => segment.toLowerCase());
    if (segments.length === 0) continue;

    let cursor = config;
    for (let index = 0; index < segments.length - 1; index += 1) {
      const key = segments[index];
      if (!isContainer(cursor[key])) {
        cursor[key] = {};
      }
      cursor = cursor[key];
    }
    cursor[segments[segments.length - 1]] = env[name];
  }

  return config;
}

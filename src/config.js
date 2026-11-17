import path from 'node:path';

export const DEFAULT_GRACE_MS = 24 * 60 * 60 * 1000;
export const DEFAULT_CACHE_SIZE = 256;
export const DEVELOPMENT_PEPPER = 'keyladder-development-pepper';

export function loadConfig(overrides = {}) {
  return {
    dataFile:
      overrides.dataFile ??
      process.env.KEYLADDER_DATA_FILE ??
      path.resolve('data', 'keys.json'),
    pepper:
      overrides.pepper ??
      process.env.KEYLADDER_PEPPER ??
      DEVELOPMENT_PEPPER,
    graceMs: Number(
      overrides.graceMs ?? process.env.KEYLADDER_GRACE_MS ?? DEFAULT_GRACE_MS,
    ),
    cacheSize: Number(
      overrides.cacheSize ?? process.env.KEYLADDER_CACHE_SIZE ?? DEFAULT_CACHE_SIZE,
    ),
    logLevel: overrides.logLevel ?? process.env.KEYLADDER_LOG_LEVEL ?? 'info',
  };
}

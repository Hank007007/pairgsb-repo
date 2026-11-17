import { createCache } from './cache.js';
import { createIssuer } from './issue.js';
import { createRevoker } from './revoke.js';
import { createRotator } from './rotate.js';
import { createStore } from './store.js';
import { createVerifier } from './verify.js';
import { createLogger } from './logger.js';
import { loadConfig } from './config.js';
import { systemClock } from './clock.js';

export function createService(options = {}) {
  const config = loadConfig(options);
  const logger =
    options.logger ??
    createLogger({ level: config.logLevel, sink: options.sink ?? console });
  const clock = options.clock ?? systemClock;

  const store = createStore({ dataFile: config.dataFile });
  store.migrateLegacyRecords(config.pepper);
  const cache = createCache({ max: config.cacheSize });

  const issuer = createIssuer({ store, clock, pepper: config.pepper });
  const verifier = createVerifier({
    store,
    cache,
    logger,
    pepper: config.pepper,
    clock,
  });
  const rotator = createRotator({
    store,
    clock,
    graceMs: config.graceMs,
    issuer,
    cache,
  });
  const revoker = createRevoker({ store, clock, cache });

  return {
    config,
    store,
    cache,
    issue: issuer.issue,
    verify: verifier.verify,
    rotate: rotator.rotate,
    revoke: revoker.revoke,
    list() {
      return store.all();
    },
  };
}

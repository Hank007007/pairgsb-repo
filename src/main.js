import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadConfig } from './config.js';
import { loadKeySet } from './jwt.js';
import { createProxy } from './proxy.js';

const DEFAULT_CONFIG = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
  'config',
  'routes.json',
);

const logger = {
  info: (message) => console.log(`[hopguard] ${message}`),
  warn: (message) => console.warn(`[hopguard] ${message}`),
  error: (message, error) => console.error(`[hopguard] ${message}`, error ?? ''),
};

export async function startGateway(configPath = process.env.HOPGUARD_CONFIG ?? DEFAULT_CONFIG) {
  const config = await loadConfig(configPath);
  const jwksPath = path.resolve(config.baseDir, config.token.jwksPath);
  const keys = await loadKeySet(jwksPath);

  const server = createProxy({
    routes: config.routes,
    upstreams: config.upstreams,
    defaultRoute: config.defaultRoute,
    keys,
    token: config.token,
    requestTimeoutMs: 10_000,
    logger,
  });

  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(config.listen.port, config.listen.host, resolve);
  });

  const address = server.address();
  logger.info(`listening on ${address.address}:${address.port}`);
  return server;
}

const invokedDirectly =
  process.argv[1] !== undefined &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);

if (invokedDirectly) {
  startGateway().catch((error) => {
    logger.error('failed to start', error);
    process.exitCode = 1;
  });
}

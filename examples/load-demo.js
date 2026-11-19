import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { load } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));

const { config, errors } = load({
  defaults: {
    service: { name: 'orders', port: 8080 },
    db: { host: 'db.internal', pool: { size: 4, timeoutMs: 1000 } },
    features: ['beta'],
    logging: { level: 'info' },
  },
  layers: [
    resolve(here, 'config/base.json'),
    resolve(here, 'config/production.json'),
  ],
  env: {
    APP_DB__POOL__REGION: 'eu-west-1',
    APP_LOGGING__LEVEL: 'warn',
  },
  prefix: 'APP_',
  argv: ['--set', 'service.owner=platform', '--set', 'logging.format=json'],
});

console.log(JSON.stringify(config, null, 2));
if (errors.length > 0) {
  console.error('errors:', errors);
  process.exitCode = 1;
}

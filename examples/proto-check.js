import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { load } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));

const { config, errors } = load({
  defaults: { service: { name: 'orders' } },
  layers: [resolve(here, 'fixtures/hostile-base.json')],
  env: {
    APP_CONSTRUCTOR__PROTOTYPE__ISADMIN: '1',
  },
  prefix: 'APP_',
  argv: ['--set', 'service.__proto__.isAdmin=1'],
});

console.log('config:', JSON.stringify(config));
console.log('errors:', JSON.stringify(errors));

const leaked = [
  ['({}).polluted', {}.polluted],
  ['({}).isAdmin', {}.isAdmin],
  ['Object.prototype.polluted', Object.prototype.polluted],
  ['Object.prototype.isAdmin', Object.prototype.isAdmin],
  ['({}).x', {}.x],
];

let dirty = false;
for (const [label, value] of leaked) {
  const state = value === undefined ? 'undefined' : JSON.stringify(value);
  if (value !== undefined) dirty = true;
  console.log(`${label} = ${state}`);
}

console.log(`global object clean: ${!dirty}`);
if (dirty) process.exitCode = 1;

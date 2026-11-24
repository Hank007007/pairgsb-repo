import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { createRuntime } from '../src/runtime.js';
import { createService } from '../src/service.js';

const here = dirname(fileURLToPath(import.meta.url));
const namespaceDir = join(here, 'fixtures', 'namespaces');

export function loadNamespace(name) {
  return JSON.parse(readFileSync(join(namespaceDir, `${name}.json`), 'utf8'));
}

/**
 * Build a service for a fresh runtime with the named fixtures registered.
 */
export function makeService(tenant = 'acme', namespaces = ['doc', 'group', 'folder']) {
  const runtime = createRuntime();
  const service = createService(runtime, tenant);
  for (const name of namespaces) {
    service.registerNamespace(loadNamespace(name));
  }
  return service;
}

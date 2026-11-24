/**
 * Namespace registry.
 *
 * Each tenant owns a registry space holding the current document for every
 * namespace name it has registered, plus the compiled plans produced from
 * those documents.
 */

import { AuthError, ErrorCodes } from '../errors.js';
import { isValidType } from '../tuple.js';
import { compileNamespace } from './compile.js';

const TENANT_PATTERN = /^[a-z0-9][a-z0-9_-]*$/i;

export function isValidTenant(tenant) {
  return typeof tenant === 'string' && TENANT_PATTERN.test(tenant);
}

export function createRegistry() {
  return { spaces: new Map() };
}

/**
 * Look up (and optionally create) a tenant's registry space.
 * @param {{spaces: Map}} registry
 * @param {string} tenant
 * @param {boolean} [create]
 */
export function getTenantSpace(registry, tenant, create = false) {
  if (!isValidTenant(tenant)) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, `invalid tenant "${tenant}"`, { tenant });
  }
  let space = registry.spaces.get(tenant);
  if (!space && create) {
    space = { tenant, documents: new Map(), compiled: new Map() };
    registry.spaces.set(tenant, space);
  }
  return space;
}

/**
 * Register (or reload) a namespace document for a tenant.
 *
 * @param {{spaces: Map}} registry
 * @param {string} tenant
 * @param {{name: string, version?: number, relations: object}} document
 * @returns {{name: string, version: number}}
 */
export function registerNamespace(registry, tenant, document) {
  if (!document || typeof document !== 'object' || Array.isArray(document)) {
    throw new AuthError(ErrorCodes.INVALID_NAMESPACE, 'namespace document must be an object');
  }
  const { name } = document;
  if (!isValidType(name)) {
    throw new AuthError(ErrorCodes.INVALID_NAMESPACE, `namespace name "${name}" is invalid`, { name });
  }
  const relations = document.relations;
  if (!relations || typeof relations !== 'object' || Array.isArray(relations)) {
    throw new AuthError(
      ErrorCodes.INVALID_NAMESPACE,
      `namespace "${name}" must declare a relations object`,
      { namespace: name },
    );
  }

  const space = getTenantSpace(registry, tenant, true);
  const previous = space.documents.get(name);

  let version;
  if (document.version === undefined) {
    version = previous ? previous.version + 1 : 1;
  } else {
    if (!Number.isInteger(document.version) || document.version < 1) {
      throw new AuthError(
        ErrorCodes.INVALID_NAMESPACE,
        `namespace "${name}" version must be a positive integer`,
        { namespace: name, version: document.version },
      );
    }
    version = document.version;
  }

  if (previous && version <= previous.version) {
    throw new AuthError(
      ErrorCodes.INVALID_NAMESPACE,
      `namespace "${name}" version ${version} does not advance past ${previous.version}`,
      { namespace: name, version, previous: previous.version },
    );
  }

  const stored = { name, version, relations };
  // Loading validates the whole document. Unknown operators, malformed
  // operands and dangling relation references must fail at registration time,
  // not silently compile into some other operator.
  const plan = compileNamespace(stored);
  space.documents.set(name, stored);
  space.compiled.set(name, plan);
  return { name, version };
}

export function getNamespaceDocument(registry, tenant, name) {
  const space = getTenantSpace(registry, tenant, false);
  if (!space) return undefined;
  return space.documents.get(name);
}

export function listNamespaces(registry, tenant) {
  const space = getTenantSpace(registry, tenant, false);
  if (!space) return [];
  return [...space.documents.values()].map((doc) => ({ name: doc.name, version: doc.version }));
}

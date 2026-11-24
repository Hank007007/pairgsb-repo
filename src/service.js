/**
 * Service layer.
 *
 * Binds a runtime and a tenant together so that the HTTP handlers, the CLI and
 * tests all go through one entry point. Nothing here decides authorization
 * semantics: it delegates to the evaluator and the stores.
 */

import {
  registerNamespace as registerNamespaceInRegistry,
  listNamespaces,
} from './model/registry.js';
import { check } from './graph/check.js';
import { expand } from './graph/expand.js';
import { write, deleteTuple, read } from './store/tuples.js';
import { parseObjectRef } from './tuple.js';

/**
 * @param {object} runtime
 * @param {string} tenant
 */
export function createService(runtime, tenant) {
  return {
    tenant,
    runtime,

    registerNamespace(document) {
      return registerNamespaceInRegistry(runtime.namespaces, tenant, document);
    },

    listNamespaces() {
      return listNamespaces(runtime.namespaces, tenant);
    },

    check(object, relation, subject, options = {}) {
      return check(runtime, tenant, object, relation, subject, options);
    },

    expand(object, relation, options = {}) {
      return expand(runtime, tenant, object, relation, options);
    },

    write(input) {
      return write(runtime.tuples, tenant, input);
    },

    delete(input) {
      return deleteTuple(runtime.tuples, tenant, input);
    },

    readTuples(object, relation) {
      const parsed = parseObjectRef(object);
      return read(runtime.tuples, tenant, parsed, relation);
    },
  };
}

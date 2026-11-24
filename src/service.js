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
  getNamespaceDocument,
} from './model/registry.js';
import { check } from './graph/check.js';
import { expand } from './graph/expand.js';
import { write, deleteTuple, read } from './store/tuples.js';
import { parseObjectRef, parseTuple } from './tuple.js';
import { AuthError, ErrorCodes } from './errors.js';

function hasRelation(document, relation) {
  return Object.prototype.hasOwnProperty.call(document.relations, relation);
}

function validateWrite(runtime, tenant, input) {
  const tuple = parseTuple(input, ErrorCodes.INVALID_TUPLE);

  const objectNamespace = getNamespaceDocument(runtime.namespaces, tenant, tuple.object.type);
  if (!objectNamespace) {
    throw new AuthError(
      ErrorCodes.NAMESPACE_NOT_FOUND,
      `namespace "${tuple.object.type}" is not registered in tenant "${tenant}"`,
      { tenant, namespace: tuple.object.type, tuple: tuple.text },
    );
  }
  if (!hasRelation(objectNamespace, tuple.relation)) {
    throw new AuthError(
      ErrorCodes.INVALID_TUPLE,
      `namespace "${tuple.object.type}" has no relation "${tuple.relation}"`,
      { tenant, namespace: tuple.object.type, relation: tuple.relation, tuple: tuple.text },
    );
  }

  if (tuple.subject.kind === 'userset') {
    const subjectNamespace = getNamespaceDocument(runtime.namespaces, tenant, tuple.subject.type);
    if (!subjectNamespace) {
      throw new AuthError(
        ErrorCodes.NAMESPACE_NOT_FOUND,
        `namespace "${tuple.subject.type}" is not registered in tenant "${tenant}"`,
        { tenant, namespace: tuple.subject.type, tuple: tuple.text },
      );
    }
    if (!hasRelation(subjectNamespace, tuple.subject.relation)) {
      throw new AuthError(
        ErrorCodes.INVALID_TUPLE,
        `namespace "${tuple.subject.type}" has no relation "${tuple.subject.relation}"`,
        {
          tenant,
          namespace: tuple.subject.type,
          relation: tuple.subject.relation,
          tuple: tuple.text,
        },
      );
    }
  }

  return tuple;
}

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
      const tuple = validateWrite(runtime, tenant, input);
      return write(runtime.tuples, tenant, tuple.text);
    },

    delete(input) {
      const tuple = validateWrite(runtime, tenant, input);
      return deleteTuple(runtime.tuples, tenant, tuple.text);
    },

    readTuples(object, relation) {
      const parsed = parseObjectRef(object);
      return read(runtime.tuples, tenant, parsed, relation);
    },
  };
}

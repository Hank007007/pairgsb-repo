/**
 * HTTP endpoints for namespace administration and tuple writes.
 *
 * POST /namespaces   { name, relations, version? } -> { name, version }
 * GET  /namespaces                               -> { namespaces: [{name, version}] }
 * POST /tuples       { tuple | object+relation+subject } -> { tuple, zookie }
 * POST /tuples/delete{ tuple | object+relation+subject } -> { tuple, zookie, deleted }
 */

import { AuthError, ErrorCodes } from '../errors.js';

function tupleFromBody(body) {
  if (typeof body.tuple === 'string') return body.tuple;
  if (body.object !== undefined || body.relation !== undefined || body.subject !== undefined) {
    return { object: body.object, relation: body.relation, subject: body.subject };
  }
  throw new AuthError(ErrorCodes.INVALID_TUPLE, 'request must supply "tuple" or object/relation/subject');
}

export function handleRegisterNamespace(service, body) {
  if (!body || typeof body !== 'object') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body must be an object');
  }
  const registered = service.registerNamespace({
    name: body.name,
    version: body.version,
    relations: body.relations,
  });
  return registered;
}

export function handleListNamespaces(service) {
  return { namespaces: service.listNamespaces() };
}

export function handleWrite(service, body) {
  if (!body || typeof body !== 'object') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body must be an object');
  }
  return service.write(tupleFromBody(body));
}

export function handleDelete(service, body) {
  if (!body || typeof body !== 'object') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body must be an object');
  }
  return service.delete(tupleFromBody(body));
}

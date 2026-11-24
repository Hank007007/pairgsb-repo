/**
 * HTTP surface.
 *
 * Routes and status codes are fixed by the service contract:
 *
 *   POST /namespaces            register a namespace document
 *   GET  /namespaces            list the tenant's namespaces
 *   POST /tuples                insert a tuple
 *   POST /tuples/delete         delete a tuple
 *   POST /check                 check membership
 *   POST /expand                enumerate members
 *   GET  /healthz               liveness
 *
 * All routes other than /healthz require an authenticated principal carrying
 * a `tenant`. The server never decides semantics itself: it parses JSON,
 * calls the service, and maps `AuthError.code` onto a status code.
 */

import { createServer as createHttpServer } from 'node:http';
import { AuthError, ErrorCodes } from '../errors.js';
import { handleCheck } from './check.js';
import { handleExpand } from './expand.js';
import { handleWrite, handleDelete, handleRegisterNamespace, handleListNamespaces } from './tuples.js';

const STATUS_BY_CODE = Object.freeze({
  [ErrorCodes.NAMESPACE_NOT_FOUND]: 404,
  [ErrorCodes.INVALID_NAMESPACE]: 400,
  [ErrorCodes.INVALID_TUPLE]: 400,
  [ErrorCodes.INVALID_REQUEST]: 400,
  [ErrorCodes.CYCLE_DETECTED]: 409,
  [ErrorCodes.DEPTH_EXCEEDED]: 409,
  [ErrorCodes.TENANT_MISMATCH]: 403,
  [ErrorCodes.TUPLE_STORE_ERROR]: 500,
});

export function statusForError(error) {
  if (error instanceof AuthError) {
    return STATUS_BY_CODE[error.code] ?? 500;
  }
  return 500;
}

function sendJson(response, status, payload) {
  const text = JSON.stringify(payload);
  response.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(text),
  });
  response.end(text);
}

async function readBody(request) {
  const chunks = [];
  let size = 0;
  for await (const chunk of request) {
    size += chunk.length;
    if (size > 1_048_576) {
      throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body is too large');
    }
    chunks.push(chunk);
  }
  const text = Buffer.concat(chunks).toString('utf8');
  if (text.trim() === '') return {};
  try {
    return JSON.parse(text);
  } catch {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body is not valid JSON');
  }
}

/**
 * @param {{serviceFor: (principal: object) => object, authenticate: (request: object) => object}} options
 */
export function createServer(options) {
  const { serviceFor, authenticate } = options;

  return createHttpServer(async (request, response) => {
    const url = new URL(request.url, 'http://localhost');
    const route = `${request.method} ${url.pathname}`;

    if (route === 'GET /healthz') {
      sendJson(response, 200, { ok: true });
      return;
    }

    let principal;
    try {
      principal = await authenticate(request);
    } catch (error) {
      sendJson(response, statusForError(error), error.toJSON?.() ?? { error: 'unauthenticated' });
      return;
    }
    if (!principal || !principal.tenant) {
      sendJson(response, 403, { error: ErrorCodes.TENANT_MISMATCH, message: 'no tenant for principal' });
      return;
    }

    const service = serviceFor(principal);

    try {
      const body = request.method === 'GET' ? {} : await readBody(request);

      switch (route) {
        case 'POST /namespaces':
          sendJson(response, 200, handleRegisterNamespace(service, body));
          return;
        case 'GET /namespaces':
          sendJson(response, 200, handleListNamespaces(service));
          return;
        case 'POST /tuples':
          sendJson(response, 200, handleWrite(service, body));
          return;
        case 'POST /tuples/delete':
          sendJson(response, 200, handleDelete(service, body));
          return;
        case 'POST /check':
          sendJson(response, 200, handleCheck(service, body, principal));
          return;
        case 'POST /expand':
          sendJson(response, 200, handleExpand(service, body, principal));
          return;
        default:
          sendJson(response, 404, { error: 'not_found', message: `no route for ${route}` });
      }
    } catch (error) {
      if (error instanceof AuthError) {
        sendJson(response, statusForError(error), error.toJSON());
        return;
      }
      sendJson(response, 500, { error: ErrorCodes.TUPLE_STORE_ERROR, message: 'internal error' });
    }
  });
}

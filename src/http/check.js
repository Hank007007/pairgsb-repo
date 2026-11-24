/**
 * HTTP check endpoint.
 *
 * POST /check
 *   { tenant, object, relation, subject, at_least?, contextualTuples? }
 * -> { allowed: boolean }
 *
 * The tenant is authenticated by the server middleware and handed in as
 * `principal.tenant`; a body that names a different tenant is rejected.
 */

import { AuthError, ErrorCodes } from '../errors.js';

function requireString(body, field) {
  const value = body[field];
  if (typeof value !== 'string' || value.length === 0) {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, `"${field}" must be a non-empty string`, { field });
  }
  return value;
}

/**
 * @param {object} service
 * @param {object} body parsed JSON request body
 * @param {{tenant: string}} principal
 */
export function handleCheck(service, body, principal) {
  if (!body || typeof body !== 'object') {
    throw new AuthError(ErrorCodes.INVALID_REQUEST, 'request body must be an object');
  }

  if (body.tenant !== undefined && body.tenant !== principal.tenant) {
    throw new AuthError(ErrorCodes.TENANT_MISMATCH, 'body tenant does not match the authenticated principal', {
      tenant: body.tenant,
    });
  }

  const object = requireString(body, 'object');
  const relation = requireString(body, 'relation');
  const subject = requireString(body, 'subject');

  const options = {};
  if (body.at_least !== undefined) options.atLeast = body.at_least;
  if (body.contextualTuples !== undefined) options.contextualTuples = body.contextualTuples;

  const allowed = service.check(object, relation, subject, options);
  return { allowed };
}

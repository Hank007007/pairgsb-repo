/**
 * HTTP expand endpoint.
 *
 * POST /expand
 *   { tenant, object, relation, at_least?, contextualTuples? }
 * -> { object, relation, subjects: string[] }
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
 * @param {object} body
 * @param {{tenant: string}} principal
 */
export function handleExpand(service, body, principal) {
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

  const options = {};
  if (body.contextualTuples !== undefined) options.contextualTuples = body.contextualTuples;

  return service.expand(object, relation, options);
}

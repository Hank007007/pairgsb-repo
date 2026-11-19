'use strict';

const { CertbraceError } = require('./errors');

/**
 * Upper bound for the lifetime of any certificate this service issues.
 * Expressed in seconds, matching the OpenSSH wire format.
 */
const MAX_CERT_TTL_SECONDS = 8 * 60 * 60;

/**
 * role -> { principals, ttlSeconds }
 *
 * `principals` is the complete set of login names a certificate with that
 * role may carry. `ttlSeconds` is the lifetime used when the request does not
 * ask for a specific one. Both are part of the contract with the
 * infrastructure team; see docs/sshca-spec.md section 2.
 */
const ROLES = Object.freeze({
  breakglass: Object.freeze({ principals: Object.freeze(['root']), ttlSeconds: 900 }),
  ci: Object.freeze({ principals: Object.freeze(['ci', 'deploy']), ttlSeconds: 1800 }),
  deploy: Object.freeze({ principals: Object.freeze(['deploy']), ttlSeconds: 3600 }),
  ops: Object.freeze({ principals: Object.freeze(['ops']), ttlSeconds: 7200 }),
  sre: Object.freeze({ principals: Object.freeze(['sre', 'ops']), ttlSeconds: 14400 }),
});

const ROLE_NAMES = Object.freeze(Object.keys(ROLES));

function getRole(name) {
  if (typeof name !== 'string' || name.length === 0) {
    throw new CertbraceError('request is missing a role', {
      code: 'invalid_role',
      status: 400,
    });
  }
  if (!Object.prototype.hasOwnProperty.call(ROLES, name)) {
    throw new CertbraceError(`unknown role: ${name}`, {
      code: 'invalid_role',
      status: 400,
      details: { role: name, knownRoles: ROLE_NAMES.slice() },
    });
  }
  return ROLES[name];
}

function allowedPrincipals(name) {
  return getRole(name).principals.slice();
}

function defaultTtlSeconds(name) {
  return getRole(name).ttlSeconds;
}

module.exports = {
  MAX_CERT_TTL_SECONDS,
  ROLES,
  ROLE_NAMES,
  allowedPrincipals,
  defaultTtlSeconds,
  getRole,
};

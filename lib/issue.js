'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const { CertbraceError } = require('./errors');
const format = require('./format');
const policy = require('./policy');
const ca = require('./ca');
const verify = require('./verify').selfCheck;

const ROOT_DIR = path.resolve(__dirname, '..');
const DEFAULT_SERIAL_FILE = path.join(ROOT_DIR, 'var', 'serial.seq');

const CRITICAL_OPTION_ALIASES = Object.freeze({
  force_command: 'force_command',
});

/**
 * Pull the request fields out of the various spellings the platform is
 * allowed to send. Values are left untouched here; validation happens when
 * the certificate is assembled.
 */
function normalizeRequest(input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    throw new CertbraceError('request body must be a JSON object', {
      code: 'invalid_request',
      status: 400,
    });
  }

  const publicKey = input.publicKey !== undefined ? input.publicKey : input.public_key;
  if (publicKey === undefined || publicKey === null) {
    throw new CertbraceError('request is missing a public key', {
      code: 'invalid_public_key',
      status: 400,
    });
  }

  policy.getRole(input.role);

  return {
    role: input.role,
    publicKey,
    principals: input.principals,
    ttlSeconds: input.ttlSeconds !== undefined ? input.ttlSeconds : input.ttl_seconds,
    criticalOptions:
      input.criticalOptions !== undefined ? input.criticalOptions : input.critical_options,
  };
}

/**
 * The principal names a certificate carries. A request that names its
 * principals gets exactly those names; otherwise the role's table entry
 * supplies the default set.
 */
function resolvePrincipals(role, requested) {
  const allowed = policy.allowedPrincipals(role);
  if (requested === undefined || requested === null) {
    return allowed;
  }
  if (!Array.isArray(requested)) {
    throw new CertbraceError('principals must be an array of strings', {
      code: 'invalid_principals',
      status: 400,
    });
  }
  const names = requested.map((name) => {
    if (typeof name !== 'string' || name.length === 0) {
      throw new CertbraceError('principals must be an array of non-empty strings', {
        code: 'invalid_principals',
        status: 400,
      });
    }
    return name;
  });
  return names;
}

/**
 * Collect the critical options for the certificate. Only options understood
 * by this service are carried over; everything else is not part of the
 * certificate.
 */
function normalizeOptions(options) {
  const result = {};
  if (options === undefined || options === null) {
    return result;
  }
  if (typeof options !== 'object' || Array.isArray(options)) {
    throw new CertbraceError('critical options must be an object', {
      code: 'invalid_options',
      status: 400,
    });
  }

  for (const key of Object.keys(options)) {
    if (CRITICAL_OPTION_ALIASES[key] === undefined) {
      continue;
    }
    const value = options[key];
    if (value === undefined || value === null) {
      continue;
    }
    result[CRITICAL_OPTION_ALIASES[key]] = String(value);
  }
  return result;
}

/**
 * Work out the validity window. `now` is milliseconds since the epoch; the
 * certificate carries seconds since the epoch.
 */
function resolveValidity(role, requestedTtlSeconds, now) {
  const ttlSeconds =
    requestedTtlSeconds === undefined || requestedTtlSeconds === null
      ? policy.defaultTtlSeconds(role)
      : Number(requestedTtlSeconds);

  if (!Number.isFinite(ttlSeconds) || ttlSeconds <= 0) {
    throw new CertbraceError('ttlSeconds must be a positive number', {
      code: 'invalid_ttl',
      status: 400,
    });
  }

  const validAfter = Math.floor(now / 1000);
  const validBefore = validAfter + Math.floor(ttlSeconds);

  return {
    validAfter: BigInt(validAfter),
    validBefore: BigInt(validBefore),
    ttlSeconds: Math.floor(ttlSeconds),
  };
}

/**
 * Allocate the next certificate serial number. Serials are persisted so that
 * a restart of the service does not reissue numbers that are still present in
 * the revocation list.
 */
function allocateSerial({ serialFile = DEFAULT_SERIAL_FILE } = {}) {
  let previous = 0n;
  try {
    const stored = fs.readFileSync(serialFile, 'utf8').trim();
    if (stored.length > 0) {
      previous = BigInt(stored);
    }
  } catch {
    previous = 0n;
  }

  const next = previous + 1n;

  try {
    fs.mkdirSync(path.dirname(serialFile), { recursive: true });
    fs.writeFileSync(serialFile, `${next}\n`, 'utf8');
  } catch {
    // The serial is still unique in this process even when the state file is
    // not writable.
  }

  return next;
}

/**
 * Assemble the certificate fields. The result still needs to be signed and
 * encoded; buildCertificate is also used by callers that want to inspect the
 * fields before signing.
 */
function buildCertificate(request, context = {}) {
  const normalized = normalizeRequest(request);
  const now = context.now === undefined ? Date.now() : context.now;
  const signingKey = context.caKey;
  if (!signingKey || !signingKey.privateKey) {
    throw new CertbraceError('a CA key is required to build a certificate', {
      code: 'ca_unavailable',
      status: 500,
    });
  }

  const subject = format.parsePublicKey(normalized.publicKey);
  const principals = resolvePrincipals(normalized.role, normalized.principals);
  const validity = resolveValidity(normalized.role, normalized.ttlSeconds, now);
  const criticalOptions = normalizeOptions(normalized.criticalOptions);
  const serial = context.serial === undefined ? allocateSerial({ serialFile: context.serialFile }) : BigInt(context.serial);
  const keyId = `${signingKey.kid}:${normalized.role}:${crypto.randomBytes(8).toString('hex')}`;

  return {
    keyType: format.ED25519_CERT_TYPE,
    nonce: crypto.randomBytes(32),
    publicKey: subject,
    serial,
    certType: format.CERT_TYPE_USER,
    keyId,
    principals,
    validAfter: validity.validAfter,
    validBefore: validity.validBefore,
    ttlSeconds: validity.ttlSeconds,
    criticalOptions,
    extensions: {},
    signatureKey: {
      blob: signingKey.publicKeyBlob,
      privateKey: signingKey.privateKey,
    },
    role: normalized.role,
  };
}

/**
 * Issue a user certificate.
 *
 * @param {object} options
 * @param {object} options.request      request body from the platform
 * @param {string} [options.caDir]      directory holding the CA keys
 * @param {object} [options.caKey]      already loaded CA key (skips selection)
 * @param {number} [options.now]        milliseconds since the epoch
 * @param {bigint|number|string} [options.serial]
 * @param {string} [options.serialFile]
 * @returns {{ certificate: object, text: string, ca: object, selfCheck: object }}
 */
function issueCertificate(options = {}) {
  const now = options.now === undefined ? Date.now() : options.now;
  const signingKey = options.caKey || ca.selectCaKey({ caDir: options.caDir });

  const certificate = buildCertificate(options.request, {
    caKey: signingKey,
    now,
    serialFile: options.serialFile,
  });

  const body = format.encodeCertificateBody(certificate);
  certificate.signature = ca.signBody(signingKey, body);

  const text = format.formatCertificate(certificate, `certbrace:${certificate.role}`);
  const selfCheck = verify(text, signingKey.publicKey, { now });

  return {
    certificate,
    text,
    ca: {
      kid: signingKey.kid,
      publicKey: signingKey.publicKey,
      publicKeyBlob: signingKey.publicKeyBlob,
    },
    selfCheck,
  };
}

module.exports = {
  CRITICAL_OPTION_ALIASES,
  allocateSerial,
  buildCertificate,
  issueCertificate,
  normalizeOptions,
  normalizeRequest,
  resolvePrincipals,
  resolveValidity,
};

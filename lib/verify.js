'use strict';

const crypto = require('node:crypto');

const { CertbraceError } = require('./errors');
const format = require('./format');
const policy = require('./policy');

/**
 * The signature field on the wire is a string containing
 * (string algorithm, string signature). Return the algorithm name and the
 * raw signature.
 */
function decodeSignature(certificate) {
  const blob = Buffer.isBuffer(certificate.signature)
    ? certificate.signature
    : Buffer.from(certificate.signature || []);
  const algorithm = readField(blob, 0);
  const signature = readField(blob, algorithm.end);
  return {
    algorithm: algorithm.value.toString('utf8'),
    signature: signature.value,
  };
}

function readField(buffer, offset) {
  if (offset + 4 > buffer.length) {
    throw new CertbraceError('certificate signature is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const length = buffer.readUInt32BE(offset);
  const end = offset + 4 + length;
  if (end > buffer.length) {
    throw new CertbraceError('certificate signature is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  return { value: buffer.subarray(offset + 4, end), end };
}

/**
 * Verify the CA signature over the certificate body.
 */
function verifySignature(certificate, caPublicKey) {
  if (!caPublicKey) {
    throw new CertbraceError('a CA public key is required for verification', {
      code: 'invalid_argument',
      status: 500,
    });
  }
  const { algorithm, signature } = decodeSignature(certificate);
  const ok = crypto.verify(null, certificate.body, caPublicKey, signature);
  return {
    ok,
    algorithm,
    details: ok
      ? null
      : `certificate signature does not verify against CA key ${describeKey(caPublicKey)}`,
  };
}

/**
 * Confirm that the certificate is inside its validity window.
 * Certificate timestamps are seconds since the epoch, while `now` arrives
 * in milliseconds (as returned by Date.now()); the units must not be mixed.
 */
function checkValidity(certificate, now = Date.now()) {
  const validAfter = Number(certificate.validAfter);
  const validBefore = Number(certificate.validBefore);
  const nowSeconds = Math.floor(Number(now) / 1000);

  if (validAfter > nowSeconds) {
    return {
      ok: false,
      reason: 'not_yet_valid',
      validAfter,
      validBefore,
      now,
      nowSeconds,
    };
  }

  if (validBefore <= nowSeconds) {
    return {
      ok: false,
      reason: 'expired',
      validAfter,
      validBefore,
      now,
      nowSeconds,
    };
  }

  return { ok: true, validAfter, validBefore, now, nowSeconds };
}

/**
 * Confirm that the certificate carries a sensible lifetime that stays
 * inside the service-wide ceiling from the policy table.
 */
function checkLifetime(certificate, maxTtlSeconds = policy.MAX_CERT_TTL_SECONDS) {
  const validAfter = Number(certificate.validAfter);
  const validBefore = Number(certificate.validBefore);
  if (!Number.isFinite(validAfter) || !Number.isFinite(validBefore) || validBefore <= validAfter) {
    return { ok: false, reason: 'invalid_window', validAfter, validBefore };
  }
  const ttlSeconds = validBefore - validAfter;
  if (maxTtlSeconds !== undefined && ttlSeconds > maxTtlSeconds) {
    return { ok: false, reason: 'ttl_exceeded', ttlSeconds, maxTtlSeconds };
  }
  return { ok: true, ttlSeconds };
}

/**
 * Compare two public keys without depending on their PEM rendering.
 */
function asPublicKey(key) {
  if (!key) {
    return null;
  }
  if (typeof key.export === 'function' && key.type === 'public') {
    return key;
  }
  return crypto.createPublicKey(key);
}

function samePublicKey(a, b) {
  const left = asPublicKey(a);
  const right = asPublicKey(b);
  if (!left || !right) {
    return false;
  }
  return left.export({ type: 'spki', format: 'der' }).equals(right.export({ type: 'spki', format: 'der' }));
}

function describeKey(key) {
  const jwk = key.export({ format: 'jwk' });
  return `${key.asymmetricKeyType}:${String(jwk.x || '').slice(0, 12)}`;
}

/**
 * Parse a certificate line and check its signature and validity window.
 * Returns a report rather than throwing so callers can decide how to react.
 */
function selfCheck(certificateText, caPublicKey, options = {}) {
  const now = options.now === undefined ? Date.now() : options.now;
  const report = { ok: false, errors: [], checks: {} };

  let certificate;
  try {
    certificate = format.parseCertificate(certificateText);
  } catch (error) {
    report.errors.push(`parse: ${error.message}`);
    return report;
  }
  report.certificate = certificate;

  try {
    const signature = verifySignature(certificate, caPublicKey);
    report.checks.signature = signature;
    if (!signature.ok) {
      report.errors.push(`signature: ${signature.details}`);
    }
  } catch (error) {
    report.checks.signature = { ok: false, details: error.message };
    report.errors.push(`signature: ${error.message}`);
  }

  const validity = checkValidity(certificate, now);
  report.checks.validity = validity;
  if (!validity.ok) {
    report.errors.push(`validity: ${validity.reason}`);
  }

  const lifetime = checkLifetime(certificate);
  report.checks.lifetime = lifetime;
  if (!lifetime.ok) {
    report.errors.push(`lifetime: ${lifetime.reason}`);
  }

  report.ok = report.errors.length === 0;
  return report;
}

module.exports = {
  checkLifetime,
  checkValidity,
  decodeSignature,
  samePublicKey,
  selfCheck,
  verifySignature,
};

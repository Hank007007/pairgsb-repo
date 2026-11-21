'use strict';

const crypto = require('node:crypto');

const {
  MANIFEST_FIELDS,
  SIGNATURE_FIELD,
  hmac,
  parseManifest,
  signingInput,
  withoutSignature,
} = require('../protocol/manifest');

class VerifyError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'VerifyError';
    this.code = details.code || 'verify_error';
    this.field = details.field;
    this.origin = details.origin;
    this.keyId = details.keyId;
  }
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function checkShape(manifest) {
  if (!isPlainObject(manifest)) {
    throw new VerifyError('manifest must be a JSON object', {
      code: 'manifest_not_object',
      field: 'manifest',
    });
  }
  for (const field of MANIFEST_FIELDS) {
    if (!(field in manifest)) {
      throw new VerifyError(`manifest is missing field ${field}`, {
        code: 'manifest_missing_field',
        field,
        origin: manifest.origin,
        keyId: manifest.keyId,
      });
    }
  }
  if (typeof manifest.origin !== 'string' || manifest.origin.length === 0) {
    throw new VerifyError('manifest origin must be a non-empty string', {
      code: 'manifest_invalid_field',
      field: 'origin',
      origin: manifest.origin,
    });
  }
  if (!Number.isInteger(manifest.epoch) || manifest.epoch < 0) {
    throw new VerifyError('manifest epoch must be a non-negative integer', {
      code: 'manifest_invalid_field',
      field: 'epoch',
      origin: manifest.origin,
    });
  }
  if (!Number.isInteger(manifest.firstSeq) || manifest.firstSeq < 0) {
    throw new VerifyError('manifest firstSeq must be a non-negative integer', {
      code: 'manifest_invalid_field',
      field: 'firstSeq',
      origin: manifest.origin,
    });
  }
  if (!Number.isInteger(manifest.count) || manifest.count <= 0) {
    throw new VerifyError('manifest count must be a positive integer', {
      code: 'manifest_invalid_field',
      field: 'count',
      origin: manifest.origin,
    });
  }
  if (typeof manifest.digest !== 'string' || manifest.digest.length === 0) {
    throw new VerifyError('manifest digest must be a non-empty string', {
      code: 'manifest_invalid_field',
      field: 'digest',
      origin: manifest.origin,
    });
  }
  if (typeof manifest.keyId !== 'string' || manifest.keyId.length === 0) {
    throw new VerifyError('manifest keyId must be a non-empty string', {
      code: 'manifest_invalid_field',
      field: 'keyId',
      origin: manifest.origin,
    });
  }
}

function verifySignature(manifest, rawText, keyring) {
  const entry = keyring && keyring[manifest.keyId];
  if (!entry) {
    throw new VerifyError(`unknown keyId ${manifest.keyId}`, {
      code: 'unknown_key',
      field: 'keyId',
      origin: manifest.origin,
      keyId: manifest.keyId,
    });
  }

  const parsed = JSON.parse(rawText);
  const input = JSON.stringify(withoutSignature(parsed));
  const expected = hmac(entry.secret, input);
  const signature = manifest[SIGNATURE_FIELD];

  if (typeof signature !== 'string' || signature === '') {
    throw new VerifyError('manifest signature is missing', {
      code: 'signature_missing',
      field: 'signature',
      origin: manifest.origin,
      keyId: manifest.keyId,
    });
  }

  if (signature === expected) {
    return manifest;
  }

  throw new VerifyError(`manifest signature mismatch for origin ${manifest.origin}`, {
    code: 'signature_mismatch',
    field: 'signature',
    origin: manifest.origin,
    keyId: manifest.keyId,
  });
}

/**
 * Parses and shape-checks a manifest without verifying its signature.
 */
function readManifest(rawText) {
  let manifest;
  try {
    manifest = parseManifest(rawText);
  } catch (err) {
    throw new VerifyError('manifest is not valid JSON', {
      code: 'manifest_not_json',
      field: 'manifest',
      cause: err,
    });
  }
  checkShape(manifest);
  return manifest;
}

/**
 * Verifies the manifest carried in `rawText` and returns its parsed value.
 * Throws VerifyError with `field` describing the rejected part.
 */
function verifyManifest(rawText, options = {}) {
  const keyring = options.keyring || {};
  const manifest = readManifest(rawText);
  return verifySignature(manifest, rawText, keyring);
}

function verifyPayload(manifest, payload) {
  const body = Buffer.isBuffer(payload) ? payload : Buffer.from(String(payload), 'utf8');
  const actual = 'sha256:' + crypto.createHash('sha256').update(body).digest('hex');
  if (manifest.digest !== actual) {
    throw new VerifyError(`manifest digest mismatch for origin ${manifest.origin}`, {
      code: 'digest_mismatch',
      field: 'digest',
      origin: manifest.origin,
      keyId: manifest.keyId,
    });
  }

  const text = body.toString('utf8');
  const lines = text.split('\n').filter((line) => line.length > 0);
  if (lines.length !== manifest.count) {
    throw new VerifyError(
      `manifest count ${manifest.count} does not match ${lines.length} records for origin ${manifest.origin}`,
      {
        code: 'count_mismatch',
        field: 'count',
        origin: manifest.origin,
        keyId: manifest.keyId,
      },
    );
  }

  return lines;
}

module.exports = {
  VerifyError,
  checkShape,
  readManifest,
  signingInput,
  verifyManifest,
  verifyPayload,
};

'use strict';

const crypto = require('node:crypto');

const MANIFEST_FIELDS = Object.freeze([
  'count',
  'digest',
  'epoch',
  'firstSeq',
  'keyId',
  'origin',
]);

const SIGNATURE_FIELD = 'signature';

const SHORT_ESCAPES = Object.freeze({
  '"': '\\"',
  '\\': '\\\\',
  '\b': '\\b',
  '\f': '\\f',
  '\n': '\\n',
  '\r': '\\r',
  '\t': '\\t',
});

function canonicalString(value) {
  let out = '"';
  for (const char of value) {
    const codePoint = char.codePointAt(0);
    const short = SHORT_ESCAPES[char];
    if (short) {
      out += short;
      continue;
    }
    if (codePoint < 0x20 || codePoint > 0x7e) {
      if (codePoint <= 0xffff) {
        out += '\\u' + codePoint.toString(16).padStart(4, '0');
      } else {
        const adjusted = codePoint - 0x10000;
        const high = 0xd800 + (adjusted >> 10);
        const low = 0xdc00 + (adjusted & 0x3ff);
        out += '\\u' + high.toString(16).padStart(4, '0');
        out += '\\u' + low.toString(16).padStart(4, '0');
      }
      continue;
    }
    out += char;
  }
  return out + '"';
}

/**
 * Deterministic JSON used for manifest bytes and signing input.
 * Object keys are ordered lexicographically; strings outside printable ASCII
 * are emitted as lower-case \\uXXXX escapes; no insignificant whitespace.
 */
function canonicalJson(value) {
  if (value === null) return 'null';
  const type = typeof value;
  if (type === 'string') return canonicalString(value);
  if (type === 'boolean') return value ? 'true' : 'false';
  if (type === 'number') {
    if (!Number.isFinite(value)) {
      throw new TypeError('canonicalJson does not support non-finite numbers');
    }
    if (Object.is(value, -0)) return '0';
    return String(value);
  }
  if (Array.isArray(value)) {
    return '[' + value.map((item) => canonicalJson(item)).join(',') + ']';
  }
  if (type === 'object') {
    const keys = Object.keys(value).sort();
    const parts = [];
    for (const key of keys) {
      const item = value[key];
      if (item === undefined) {
        throw new TypeError(`canonicalJson does not support undefined (key ${key})`);
      }
      parts.push(canonicalString(key) + ':' + canonicalJson(item));
    }
    return '{' + parts.join(',') + '}';
  }
  throw new TypeError(`canonicalJson does not support ${type}`);
}

function withoutSignature(manifest) {
  const copy = {};
  for (const key of Object.keys(manifest)) {
    if (key !== SIGNATURE_FIELD) copy[key] = manifest[key];
  }
  return copy;
}

function signingInput(manifest) {
  return canonicalJson(withoutSignature(manifest));
}

function digestPayload(payload) {
  const bytes = Buffer.isBuffer(payload) ? payload : Buffer.from(String(payload), 'utf8');
  return 'sha256:' + crypto.createHash('sha256').update(bytes).digest('hex');
}

function hmac(secret, text) {
  const key = Buffer.isBuffer(secret) ? secret : Buffer.from(String(secret), 'utf8');
  return crypto.createHmac('sha256', key).update(text, 'utf8').digest('base64');
}

function buildManifest({ origin, epoch, firstSeq, count, digest, keyId }) {
  return {
    count,
    digest,
    epoch,
    firstSeq,
    keyId,
    origin,
  };
}

function signManifest(manifest, secret) {
  const signature = hmac(secret, signingInput(manifest));
  return { ...withoutSignature(manifest), [SIGNATURE_FIELD]: signature };
}

function serializeManifest(manifest) {
  return canonicalJson(manifest);
}

function parseManifest(raw) {
  const value = typeof raw === 'string' ? JSON.parse(raw) : raw;
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new TypeError('manifest must be a JSON object');
  }
  return value;
}

function streamKey(origin, epoch) {
  return `${origin}\u0000${epoch}`;
}

module.exports = {
  MANIFEST_FIELDS,
  SIGNATURE_FIELD,
  buildManifest,
  canonicalJson,
  digestPayload,
  hmac,
  parseManifest,
  serializeManifest,
  signManifest,
  signingInput,
  streamKey,
  withoutSignature,
};

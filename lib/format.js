'use strict';

const crypto = require('node:crypto');
const { CertbraceError } = require('./errors');

const ED25519_KEY_TYPE = 'ssh-ed25519';
const ED25519_CERT_TYPE = 'ssh-ed25519-cert-v01@openssh.com';
const CERT_TYPE_USER = 1;

/** SPKI DER prefix for a raw Ed25519 public key (RFC 8410). */
const ED25519_SPKI_PREFIX = Buffer.from('302a300506032b6570032100', 'hex');

function encodeUint32(value) {
  const buffer = Buffer.alloc(4);
  buffer.writeUInt32BE(value >>> 0, 0);
  return buffer;
}

function encodeUint64(value) {
  const buffer = Buffer.alloc(8);
  buffer.writeBigUInt64BE(BigInt.asUintN(64, BigInt(value)), 0);
  return buffer;
}

/**
 * Encode a value as an SSH string: a uint32 length followed by the bytes.
 * Accepts strings and buffers.
 */
function encodeString(value) {
  const bytes = Buffer.isBuffer(value) ? value : Buffer.from(String(value), 'utf8');
  return Buffer.concat([encodeUint32(bytes.length), bytes]);
}

/**
 * A name-list on the wire is a string containing a sequence of strings.
 */
function encodeNameList(names) {
  const list = Array.isArray(names) ? names : [];
  return encodeString(Buffer.concat(list.map((name) => encodeString(name))));
}

/**
 * Serial numbers travel through the certificate as a uint64 field. The
 * allocator hands us a decimal counter; split it into the two 32-bit words
 * the field is made of and write the leading word.
 */
function encodeSerial(serial) {
  const value = BigInt.asUintN(64, BigInt(serial));
  const words = [Number(value >> 32n), Number(value & 0xffffffffn)];
  return encodeUint64(BigInt(words[0]));
}

/**
 * Critical options and extensions share the same layout: a string holding
 * a sequence of (string name, string data) tuples, ordered by name. For
 * `force_command` the data payload is the command itself.
 */
function encodeOptionList(options) {
  const entries = options && typeof options === 'object' ? options : {};
  const names = Object.keys(entries).sort();
  return Buffer.concat(
    names.map((name) => Buffer.concat([encodeString(name), encodeString(entries[name])])),
  );
}

function encodeOptions(options) {
  return encodeString(encodeOptionList(options));
}

/**
 * Decode the body of a critical-options or extensions field into a plain
 * object. The value of every entry is a string.
 */
function decodeOptionList(buffer) {
  const bytes = Buffer.isBuffer(buffer) ? buffer : Buffer.from(buffer || []);
  const entries = {};
  let cursor = 0;
  while (cursor < bytes.length) {
    const name = readStringAt(bytes, cursor);
    const value = readStringAt(bytes, name.end);
    entries[name.value.toString('utf8')] = value.value.toString('utf8');
    cursor = value.end;
  }
  return entries;
}

function encodeExtensions(extensions) {
  return encodeString(encodeOptionList(extensions));
}

function normalizePublicKeyInput(input) {
  if (Buffer.isBuffer(input)) {
    const text = input.toString('utf8').trim();
    return { text, buffer: input };
  }
  if (typeof input === 'string') {
    return { text: input.trim(), buffer: null };
  }
  return { text: null, buffer: null };
}

function rawFromKeyObject(keyObject) {
  const jwk = keyObject.export({ format: 'jwk' });
  if (jwk && typeof jwk.x === 'string') {
    return Buffer.from(jwk.x, 'base64url');
  }
  const der = keyObject.export({ type: 'spki', format: 'der' });
  return der.subarray(der.length - 32);
}

function keyObjectFromRaw(raw) {
  return crypto.createPublicKey({
    key: Buffer.concat([ED25519_SPKI_PREFIX, raw]),
    format: 'der',
    type: 'spki',
  });
}

/**
 * Accept a client public key either as an authorized_keys line
 * ("ssh-ed25519 AAAA... comment") or as an SPKI PEM block. Returns the raw
 * 32-byte Ed25519 key together with its OpenSSH wire blob and KeyObject.
 */
function parsePublicKey(input) {
  if (input && typeof input === 'object' && typeof input.export === 'function') {
    if (input.type !== 'public') {
      throw new CertbraceError('public key must be a public key', {
        code: 'invalid_public_key',
        status: 400,
      });
    }
    if (input.asymmetricKeyType !== 'ed25519') {
      throw new CertbraceError(`unsupported public key type: ${input.asymmetricKeyType}`, {
        code: 'unsupported_key_type',
        status: 400,
      });
    }
    const raw = rawFromKeyObject(input);
    return buildParsedKey(raw, input);
  }

  const { text } = normalizePublicKeyInput(input);
  if (!text) {
    throw new CertbraceError('request is missing a public key', {
      code: 'invalid_public_key',
      status: 400,
    });
  }

  if (text.startsWith('-----BEGIN')) {
    let keyObject;
    try {
      keyObject = crypto.createPublicKey(text);
    } catch {
      throw new CertbraceError('public key is not a valid PEM block', {
        code: 'invalid_public_key',
        status: 400,
      });
    }
    if (keyObject.asymmetricKeyType !== 'ed25519') {
      throw new CertbraceError(`unsupported public key type: ${keyObject.asymmetricKeyType}`, {
        code: 'unsupported_key_type',
        status: 400,
      });
    }
    return buildParsedKey(rawFromKeyObject(keyObject), keyObject);
  }

  const fields = text.split(/\s+/);
  if (fields.length < 2 || fields[0] !== ED25519_KEY_TYPE) {
    throw new CertbraceError('public key must be an ssh-ed25519 authorized_keys line', {
      code: 'invalid_public_key',
      status: 400,
    });
  }
  let blob;
  try {
    blob = Buffer.from(fields[1], 'base64');
  } catch {
    throw new CertbraceError('public key blob is not valid base64', {
      code: 'invalid_public_key',
      status: 400,
    });
  }
  if (readStringAt(blob, 0).value.toString('utf8') !== ED25519_KEY_TYPE) {
    throw new CertbraceError('public key blob is not ssh-ed25519', {
      code: 'invalid_public_key',
      status: 400,
    });
  }
  const raw = readStringAt(blob, readStringAt(blob, 0).end).value;
  if (raw.length !== 32) {
    throw new CertbraceError('ssh-ed25519 public key must be 32 bytes', {
      code: 'invalid_public_key',
      status: 400,
    });
  }
  return buildParsedKey(raw, keyObjectFromRaw(raw));
}

function buildParsedKey(raw, keyObject) {
  return {
    type: ED25519_KEY_TYPE,
    raw: Buffer.from(raw),
    blob: Buffer.concat([encodeString(ED25519_KEY_TYPE), encodeString(raw)]),
    keyObject,
  };
}

/**
 * Minimal reader for a single SSH string at `offset`. Used for key blobs and
 * for the certificate parser in this module.
 */
function readStringAt(buffer, offset) {
  if (offset + 4 > buffer.length) {
    throw new CertbraceError('certificate encoding is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const length = buffer.readUInt32BE(offset);
  const end = offset + 4 + length;
  if (end > buffer.length) {
    throw new CertbraceError('certificate encoding is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  return { value: buffer.subarray(offset + 4, end), end };
}

function readNameListAt(buffer, offset) {
  const outer = readStringAt(buffer, offset);
  const names = [];
  let cursor = 0;
  while (cursor < outer.value.length) {
    const item = readStringAt(outer.value, cursor);
    names.push(item.value.toString('utf8'));
    cursor = item.end;
  }
  return { value: names, end: outer.end };
}

/**
 * Encode everything that the CA signs: the certificate fields up to and
 * including the signing key. The signature itself is appended separately.
 */
function encodeCertificateBody(certificate) {
  if (!certificate || !certificate.signatureKey || !certificate.publicKey) {
    throw new CertbraceError('certificate is missing key material', {
      code: 'invalid_certificate',
      status: 400,
    });
  }

  return Buffer.concat([
    encodeString(certificate.keyType || ED25519_CERT_TYPE),
    encodeString(certificate.nonce || Buffer.alloc(0)),
    encodeString(certificate.publicKey.blob),
    encodeSerial(certificate.serial),
    encodeUint32(certificate.certType || CERT_TYPE_USER),
    encodeString(certificate.keyId),
    encodeNameList(certificate.principals),
    encodeUint64(certificate.validAfter),
    encodeUint64(certificate.validBefore),
    encodeOptions(certificate.criticalOptions),
    encodeExtensions(certificate.extensions),
    encodeString(Buffer.alloc(0)),
    encodeString(certificate.signatureKey.blob),
  ]);
}

/**
 * Full certificate blob: signed body followed by the string-encoded
 * signature.
 */
function encodeCertificate(certificate) {
  return Buffer.concat([encodeCertificateBody(certificate), encodeString(certificate.signature)]);
}

/**
 * Authorized-keys style rendering of a certificate. The first token is the
 * certificate key type; sshd and ssh-keygen both accept this form.
 */
function formatCertificate(certificate, comment) {
  const blob = encodeCertificate(certificate);
  const suffix = comment ? ` ${String(comment).replace(/\s+/g, '_')}` : '';
  return `${ED25519_CERT_TYPE} ${blob.toString('base64')}${suffix}\n`;
}

/**
 * Parse an authorized-keys style certificate line back into its fields.
 * Only the fields used by the self-check are decoded.
 */
function parseCertificate(text) {
  const trimmed = String(text).trim();
  const fields = trimmed.split(/\s+/);
  if (fields.length < 2 || fields[0] !== ED25519_CERT_TYPE) {
    throw new CertbraceError('certificate must be an ssh-ed25519 certificate line', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const blob = Buffer.from(fields[1], 'base64');
  let offset = 0;
  const take = () => {
    const item = readStringAt(blob, offset);
    offset = item.end;
    return item.value;
  };

  const keyType = take().toString('utf8');
  const nonce = take();
  const publicKeyBlob = take();
  if (offset + 8 > blob.length) {
    throw new CertbraceError('certificate encoding is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const serial = blob.readBigUInt64BE(offset);
  offset += 8;
  if (offset + 4 > blob.length) {
    throw new CertbraceError('certificate encoding is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const certType = blob.readUInt32BE(offset);
  offset += 4;
  const keyId = take().toString('utf8');
  const principals = readNameListAt(blob, offset);
  offset = principals.end;
  if (offset + 16 > blob.length) {
    throw new CertbraceError('certificate encoding is truncated', {
      code: 'invalid_certificate',
      status: 400,
    });
  }
  const validAfter = blob.readBigUInt64BE(offset);
  offset += 8;
  const validBefore = blob.readBigUInt64BE(offset);
  offset += 8;
  const criticalOptions = take();
  const extensions = take();
  take(); // reserved
  const signatureKeyBlob = take();
  const signature = take();
  const body = blob.subarray(0, offset - signature.length - 4);

  return {
    keyType,
    nonce,
    publicKeyBlob,
    serial,
    certType,
    keyId,
    principals: principals.value,
    validAfter,
    validBefore,
    criticalOptions,
    extensions,
    signatureKeyBlob,
    signature,
    body,
    blob,
  };
}

module.exports = {
  CERT_TYPE_USER,
  ED25519_CERT_TYPE,
  ED25519_KEY_TYPE,
  decodeOptionList,
  encodeCertificate,
  encodeCertificateBody,
  encodeNameList,
  encodeOptionList,
  encodeOptions,
  encodeSerial,
  encodeString,
  encodeUint32,
  encodeUint64,
  formatCertificate,
  keyObjectFromRaw,
  parseCertificate,
  parsePublicKey,
};

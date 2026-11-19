'use strict';

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const { CertbraceError } = require('./errors');
const { ED25519_KEY_TYPE, encodeString } = require('./format');

const DEFAULT_CA_DIR = path.resolve(__dirname, '..', 'ca');
const KEY_SUFFIX = '.key';

/**
 * The identity of a CA key is its key id: the file name without the `.key`
 * suffix. Rotation keeps the old file in place and appends `.retired` to the
 * key id, so `certbrace-v1.retired.key` is the retired form of
 * `certbrace-v1`.
 */
function keyIdFromFile(file) {
  return file.slice(0, -KEY_SUFFIX.length);
}

function isRetired(keyId) {
  return /(^|\.)retired$/.test(keyId);
}

/**
 * Parse the version number out of an active key id (`certbrace-v<N>`).
 * Retired ids and any other shape never participate in selection.
 */
function activeKeyVersion(keyId) {
  const match = /^certbrace-v(\d+)$/.exec(keyId);
  return match ? Number(match[1]) : null;
}

function readCaPublicKey(privateKey) {
  return crypto.createPublicKey(privateKey);
}

/**
 * Encode an Ed25519 public KeyObject in OpenSSH wire format.
 */
function publicKeyBlob(keyObject) {
  const jwk = keyObject.export({ format: 'jwk' });
  const raw = Buffer.from(jwk.x, 'base64url');
  return Buffer.concat([encodeString(ED25519_KEY_TYPE), encodeString(raw)]);
}

/**
 * Load one CA private key from disk. The PEM body is parsed by Node's crypto
 * module; this function only deals with the file naming contract.
 */
function loadCaKey(filePath) {
  const file = path.basename(filePath);
  if (!file.endsWith(KEY_SUFFIX)) {
    throw new CertbraceError(`not a CA key file: ${file}`, {
      code: 'invalid_ca_key',
      status: 500,
    });
  }

  const pem = fs.readFileSync(filePath, 'utf8');
  const privateKey = crypto.createPrivateKey(pem);
  if (privateKey.asymmetricKeyType !== 'ed25519') {
    throw new CertbraceError(`CA key ${file} is not ed25519`, {
      code: 'invalid_ca_key',
      status: 500,
    });
  }

  const kid = keyIdFromFile(file);
  const publicKey = readCaPublicKey(privateKey);

  return {
    kid,
    file,
    path: filePath,
    retired: isRetired(kid),
    privateKey,
    publicKey,
    publicKeyBlob: publicKeyBlob(publicKey),
  };
}

/**
 * List the CA key files present in a directory.
 */
function listCaKeyFiles(caDir = DEFAULT_CA_DIR) {
  let entries;
  try {
    entries = fs.readdirSync(caDir);
  } catch (error) {
    throw new CertbraceError(`CA directory is not readable: ${caDir}`, {
      code: 'ca_unavailable',
      status: 500,
      details: { cause: error.code },
    });
  }
  return entries.filter((entry) => entry.endsWith(KEY_SUFFIX));
}

/**
 * Choose the CA key used for signing from the CA directory. Only active
 * keys are eligible: retired keys verify historical certificates but never
 * sign new ones. Among the active keys, the highest version number wins,
 * independently of the order the filesystem returns entries.
 *
 * @returns {{ kid: string, privateKey: crypto.KeyObject, publicKey: crypto.KeyObject,
 *             publicKeyBlob: Buffer, file: string, path: string, retired: boolean }}
 */
function selectCaKey(options = {}) {
  const caDir = options.caDir || DEFAULT_CA_DIR;
  const files = listCaKeyFiles(caDir);

  const activeKeys = [];
  for (const file of files) {
    try {
      const key = loadCaKey(path.join(caDir, file));
      if (!key.retired && activeKeyVersion(key.kid) !== null) {
        activeKeys.push(key);
      }
    } catch {
      continue;
    }
  }

  if (activeKeys.length === 0) {
    throw new CertbraceError(`no usable active CA key in ${caDir}`, {
      code: 'ca_unavailable',
      status: 500,
    });
  }

  activeKeys.sort((left, right) => activeKeyVersion(right.kid) - activeKeyVersion(left.kid));
  return activeKeys[0];
}

/**
 * Sign a certificate body with the CA key. Ed25519 signatures are produced
 * over the raw bytes as a single message; the wire form wraps the signature
 * blob in a string that starts with the signature key type.
 */
function signBody(key, body) {
  const signature = crypto.sign(null, body, key.privateKey);
  return Buffer.concat([encodeString(ED25519_KEY_TYPE), encodeString(signature)]);
}

module.exports = {
  DEFAULT_CA_DIR,
  KEY_SUFFIX,
  isRetired,
  keyIdFromFile,
  listCaKeyFiles,
  loadCaKey,
  publicKeyBlob,
  selectCaKey,
  signBody,
};

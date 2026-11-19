'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

/**
 * Generate an Ed25519 key pair for a subject (certificate holder).
 */
function makeSubjectKey() {
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  return { publicKey, privateKey };
}

/**
 * Generate an Ed25519 key pair that acts as a CA, write the private key in
 * PKCS#8 PEM form under <kid>.key and return the identity of the key.
 */
function makeCaKey(dir, kid) {
  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  const pem = privateKey.export({ type: 'pkcs8', format: 'pem' });
  const file = path.join(dir, `${kid}.key`);
  fs.writeFileSync(file, pem, { mode: 0o600 });
  return { kid, privateKey, publicKey, file };
}

/**
 * A scratch directory that is removed when the test file finishes.
 */
function makeTmpDir(prefix) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `certbrace-${prefix || 'test'}-`));
  return dir;
}

function removeDir(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
}

/**
 * authorized_keys rendering of a public key, as the platform sends it.
 */
function authorizedKeysLine(publicKey, comment = 'tester') {
  return sshLine(publicKey, comment);
}

function sshLine(publicKey, comment) {
  const jwk = publicKey.export({ format: 'jwk' });
  const raw = Buffer.from(jwk.x, 'base64url');
  const type = Buffer.from('ssh-ed25519', 'utf8');
  const blob = Buffer.concat([
    uint32(type.length),
    type,
    uint32(raw.length),
    raw,
  ]);
  return `ssh-ed25519 ${blob.toString('base64')} ${comment}`;
}

function uint32(value) {
  const buffer = Buffer.alloc(4);
  buffer.writeUInt32BE(value >>> 0, 0);
  return buffer;
}

module.exports = {
  authorizedKeysLine,
  makeCaKey,
  makeSubjectKey,
  makeTmpDir,
  removeDir,
  sshLine,
  uint32,
};

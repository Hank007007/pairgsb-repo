'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

const format = require('../lib/format');
const { makeSubjectKey, authorizedKeysLine } = require('../test-support/keys');

function readUint32(buffer, offset) {
  return buffer.readUInt32BE(offset);
}

test('encodeString prefixes the payload with its length', () => {
  const encoded = format.encodeString('ssh-ed25519');
  assert.equal(readUint32(encoded, 0), 11);
  assert.equal(encoded.subarray(4).toString('utf8'), 'ssh-ed25519');
});

test('encodeUint32 writes a big-endian word', () => {
  const encoded = format.encodeUint32(0x01020304);
  assert.deepEqual([...encoded], [1, 2, 3, 4]);
});

test('encodeUint64 writes eight bytes', () => {
  const encoded = format.encodeUint64(1n);
  assert.equal(encoded.length, 8);
  assert.deepEqual([...encoded], [0, 0, 0, 0, 0, 0, 0, 1]);
});

test('encodeNameList wraps each name in a string', () => {
  const encoded = format.encodeNameList(['alice', 'bob']);
  const outerLength = readUint32(encoded, 0);
  assert.equal(outerLength, encoded.length - 4);

  let cursor = 4;
  const names = [];
  while (cursor < encoded.length) {
    const length = readUint32(encoded, cursor);
    names.push(encoded.subarray(cursor + 4, cursor + 4 + length).toString('utf8'));
    cursor += 4 + length;
  }
  assert.deepEqual(names, ['alice', 'bob']);
});

test('option lists round-trip through the decoder', () => {
  const encoded = format.encodeOptionList({
    force_command: '/usr/bin/backup',
  });
  assert.deepEqual(format.decodeOptionList(encoded), {
    force_command: '/usr/bin/backup',
  });
});

test('option lists are ordered by name', () => {
  const encoded = format.encodeOptionList({ zeta: 'z', alpha: 'a' });
  const firstLength = readUint32(encoded, 0);
  assert.equal(encoded.subarray(4, 4 + firstLength).toString('utf8'), 'alpha');
});

test('parsePublicKey accepts an authorized_keys line', () => {
  const { publicKey } = makeSubjectKey();
  const line = authorizedKeysLine(publicKey, 'alice@laptop');
  const parsed = format.parsePublicKey(line);

  assert.equal(parsed.type, 'ssh-ed25519');
  assert.equal(parsed.raw.length, 32);
  assert.equal(parsed.blob.subarray(0, 4).readUInt32BE(0), 11);
  assert.equal(parsed.keyObject.asymmetricKeyType, 'ed25519');
});

test('parsePublicKey accepts an SPKI PEM block', () => {
  const { publicKey } = makeSubjectKey();
  const pem = publicKey.export({ type: 'spki', format: 'pem' });
  const parsed = format.parsePublicKey(pem);

  assert.equal(parsed.raw.length, 32);
  assert.deepEqual(parsed.raw, parsed.keyObject.export({ format: 'jwk' }).x
    ? Buffer.from(parsed.keyObject.export({ format: 'jwk' }).x, 'base64url')
    : Buffer.alloc(0));
});

test('parsePublicKey rejects non-ed25519 keys', () => {
  const { publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 });
  assert.throws(() => format.parsePublicKey(publicKey), /unsupported public key type/);
});

test('certificate body round-trips through parseCertificate', () => {
  const subject = format.parsePublicKey(authorizedKeysLine(makeSubjectKey().publicKey));
  const caKey = crypto.generateKeyPairSync('ed25519');
  const caFormat = require('../lib/ca');

  const certificate = {
    keyType: format.ED25519_CERT_TYPE,
    nonce: crypto.randomBytes(32),
    publicKey: subject,
    serial: 42n,
    certType: format.CERT_TYPE_USER,
    keyId: 'certbrace:deploy:abc',
    principals: ['deploy'],
    validAfter: 1700000000n,
    validBefore: 1700003600n,
    criticalOptions: { force_command: '/usr/bin/backup' },
    extensions: {},
    signatureKey: {
      blob: caFormat.publicKeyBlob(crypto.createPublicKey(caKey.privateKey)),
      privateKey: caKey.privateKey,
    },
  };
  certificate.signature = caFormat.signBody(certificate.signatureKey, format.encodeCertificateBody(certificate));

  const text = format.formatCertificate(certificate, 'certbrace:deploy');
  const parsed = format.parseCertificate(text);

  assert.equal(parsed.keyId, 'certbrace:deploy:abc');
  assert.deepEqual(parsed.principals, ['deploy']);
  assert.equal(parsed.validAfter, 1700000000n);
  assert.equal(parsed.validBefore, 1700003600n);
  assert.equal(parsed.certType, 1);
  assert.deepEqual(format.decodeOptionList(parsed.criticalOptions), {
    force_command: '/usr/bin/backup',
  });
});

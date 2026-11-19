#!/usr/bin/env node
'use strict';

/**
 * Development helper: create an Ed25519 CA key pair in the CA directory.
 *
 *   node scripts/generate-ca.js certbrace-v2
 *   node scripts/generate-ca.js certbrace-v1.retired
 *
 * Writes <kid>.key (PKCS#8 PEM) and <kid>.pub (authorized_keys form). The
 * public key is what sshd loads as TrustedUserCAKeys.
 */

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const ca = require('../lib/ca');

function sshPublicKeyLine(keyObject, comment) {
  const raw = Buffer.from(keyObject.export({ format: 'jwk' }).x, 'base64url');
  const type = Buffer.from('ssh-ed25519', 'utf8');
  const length = (value) => {
    const buffer = Buffer.alloc(4);
    buffer.writeUInt32BE(value >>> 0, 0);
    return buffer;
  };
  const blob = Buffer.concat([
    length(type.length),
    type,
    length(raw.length),
    raw,
  ]);
  return `ssh-ed25519 ${blob.toString('base64')} ${comment}\n`;
}

function main(argv) {
  const kid = argv[0];
  if (!kid) {
    process.stderr.write('usage: node scripts/generate-ca.js <kid>\n');
    process.exitCode = 2;
    return;
  }
  if (!/^[A-Za-z0-9._-]+$/.test(kid)) {
    process.stderr.write('kid may only contain letters, digits, dot, dash and underscore\n');
    process.exitCode = 2;
    return;
  }

  const caDir = ca.DEFAULT_CA_DIR;
  fs.mkdirSync(caDir, { recursive: true });

  const keyFile = path.join(caDir, `${kid}.key`);
  const pubFile = path.join(caDir, `${kid}.pub`);
  if (fs.existsSync(keyFile)) {
    process.stderr.write(`${keyFile} already exists, refusing to overwrite\n`);
    process.exitCode = 1;
    return;
  }

  const { publicKey, privateKey } = crypto.generateKeyPairSync('ed25519');
  fs.writeFileSync(keyFile, privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 });
  fs.writeFileSync(pubFile, sshPublicKeyLine(publicKey, kid), { mode: 0o644 });

  process.stdout.write(`${keyFile}\n${pubFile}\n`);
}

main(process.argv.slice(2));

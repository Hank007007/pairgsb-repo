'use strict';

// Encrypted secret store.
//
// The store file holds one record per secret:
//   { "<scope>": { "<NAME>": { "value": "<base64 ciphertext>" } } }
//
// The ciphertext is a simple XOR stream keyed from the store key file. This is
// a stand-in for the runner-local decryptor, not a general-purpose cipher.

const fs = require('node:fs');

function loadKey(keyPath) {
  const raw = fs.readFileSync(keyPath, 'utf8').trim();
  return Buffer.from(raw, 'base64');
}

function decrypt(key, encoded) {
  const data = Buffer.from(encoded, 'base64');
  const out = Buffer.alloc(data.length);

  for (let i = 0; i < data.length; i += 1) {
    out[i] = data[i] ^ key[i % key.length];
  }

  return out.toString('utf8');
}

function encrypt(key, text) {
  const data = Buffer.from(text, 'utf8');
  const out = Buffer.alloc(data.length);

  for (let i = 0; i < data.length; i += 1) {
    out[i] = data[i] ^ key[i % key.length];
  }

  return out.toString('base64');
}

class Store {
  constructor(document, key) {
    this.document = document || {};
    this.key = key;
  }

  static open(storePath, keyPath) {
    const document = JSON.parse(fs.readFileSync(storePath, 'utf8'));
    const key = loadKey(keyPath);
    return new Store(document, key);
  }

  // Returns the decrypted variable map for one scope, or undefined when the
  // scope has no record in the store.
  fetch(scope) {
    const record = this.document[scope];
    if (!record) return undefined;

    const values = {};
    for (const name of Object.keys(record)) {
      const entry = record[name];
      values[name] = entry && typeof entry.value === 'string'
        ? decrypt(this.key, entry.value)
        : '';
    }
    return values;
  }

  // All variable names present in the store, across scopes.
  listNames() {
    const names = new Set();
    for (const scope of Object.keys(this.document)) {
      for (const name of Object.keys(this.document[scope])) names.add(name);
    }
    return Array.from(names).sort();
  }
}

module.exports = { Store, decrypt, encrypt };

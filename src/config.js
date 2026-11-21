'use strict';

const fs = require('node:fs');

function readJsonFile(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function loadKeyring(filePath) {
  const data = readJsonFile(filePath);
  const keyring = {};
  for (const [keyId, entry] of Object.entries(data)) {
    if (typeof entry === 'string') {
      keyring[keyId] = { secret: entry, origins: [] };
      continue;
    }
    keyring[keyId] = {
      secret: String(entry.secret),
      origins: Array.isArray(entry.origins) ? entry.origins.map(String) : [],
    };
  }
  return keyring;
}

function readSecret(filePath) {
  if (!filePath) return null;
  return fs.readFileSync(filePath, 'utf8').trim();
}

module.exports = {
  loadKeyring,
  readJsonFile,
  readSecret,
};

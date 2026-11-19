'use strict';

const fs = require('node:fs');
const path = require('node:path');

const KEY_ID_PATTERN = /^[A-Za-z0-9._:-]{1,128}$/;
const HEADER_NAME_PATTERN = /^[A-Za-z0-9!#$%&'*+.^_`|~-]+$/;

class VaultError extends Error {
  constructor(message, code) {
    super(message);
    this.name = 'VaultError';
    this.code = code || 'EVAULT';
  }
}

function normalizeHost(value) {
  if (typeof value !== 'string') {
    throw new VaultError('rule host must be a string', 'EVAULTRULE');
  }
  let host = value.trim().toLowerCase();
  if (host.endsWith('.')) host = host.slice(0, -1);
  const labels = host.split('.');
  if (
    host.length === 0 ||
    host.length > 253 ||
    labels.some(
      (label) =>
        label.length === 0 ||
        label.length > 63 ||
        !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(label)
    )
  ) {
    throw new VaultError(`invalid rule host: ${JSON.stringify(value)}`, 'EVAULTRULE');
  }
  return host;
}

function normalizeCredential(keyId, raw, baseDir) {
  if (!KEY_ID_PATTERN.test(keyId)) {
    throw new VaultError(`invalid credential key id: ${JSON.stringify(keyId)}`, 'EVAULTCRED');
  }
  if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) {
    throw new VaultError(`credential ${keyId} must be an object`, 'EVAULTCRED');
  }
  if (typeof raw.secret !== 'string' || raw.secret.length === 0) {
    throw new VaultError(`credential ${keyId} has no secret`, 'EVAULTCRED');
  }

  const header = typeof raw.header === 'string' && raw.header.length > 0 ? raw.header : 'Authorization';
  if (!HEADER_NAME_PATTERN.test(header)) {
    throw new VaultError(`credential ${keyId} has an invalid header name`, 'EVAULTCRED');
  }
  if (raw.scheme !== undefined && (typeof raw.scheme !== 'string' || raw.scheme.length === 0)) {
    throw new VaultError(`credential ${keyId} has an invalid scheme`, 'EVAULTCRED');
  }

  const extraHeaders = new Map();
  if (raw.headers !== undefined) {
    if (raw.headers === null || typeof raw.headers !== 'object' || Array.isArray(raw.headers)) {
      throw new VaultError(`credential ${keyId} headers must be an object`, 'EVAULTCRED');
    }
    for (const [name, value] of Object.entries(raw.headers)) {
      if (!HEADER_NAME_PATTERN.test(name) || typeof value !== 'string') {
        throw new VaultError(`credential ${keyId} has an invalid header entry`, 'EVAULTCRED');
      }
      extraHeaders.set(name.toLowerCase(), { name, value });
    }
  }

  let clientCert = null;
  if (raw.clientCert !== undefined) {
    const cert = raw.clientCert;
    if (cert === null || typeof cert !== 'object' || Array.isArray(cert)) {
      throw new VaultError(`credential ${keyId} clientCert must be an object`, 'EVAULTCRED');
    }
    if (typeof cert.certFile !== 'string' || typeof cert.keyFile !== 'string') {
      throw new VaultError(`credential ${keyId} clientCert needs certFile and keyFile`, 'EVAULTCRED');
    }
    clientCert = {
      certFile: path.resolve(baseDir, cert.certFile),
      keyFile: path.resolve(baseDir, cert.keyFile),
      passphrase: typeof cert.passphrase === 'string' ? cert.passphrase : undefined,
    };
  }

  return {
    keyId,
    secret: raw.secret,
    header,
    scheme: raw.scheme === undefined ? null : raw.scheme,
    headers: extraHeaders,
    clientCert,
  };
}

function normalizeRule(raw, credentials, seenHosts) {
  if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) {
    throw new VaultError('rule must be an object', 'EVAULTRULE');
  }
  if (typeof raw.host !== 'string') {
    throw new VaultError('rule is missing host', 'EVAULTRULE');
  }
  const host = normalizeHost(raw.host);
  if (raw.host.trim().toLowerCase() !== host) {
    throw new VaultError(`rule host must be a plain DNS name: ${JSON.stringify(raw.host)}`, 'EVAULTRULE');
  }
  if (seenHosts.has(host)) {
    throw new VaultError(`duplicate rule for host ${host}`, 'EVAULTRULE');
  }
  seenHosts.add(host);

  if (typeof raw.credential !== 'string' || !credentials.has(raw.credential)) {
    throw new VaultError(`rule for ${host} references an unknown credential`, 'EVAULTRULE');
  }
  return { host, credential: raw.credential };
}

/**
 * Reads and validates a vault file. Parsing is deliberately strict: a vault that
 * is ambiguous (duplicate host, unknown credential, malformed header) is a
 * start-up error rather than a partially loaded one.
 */
function loadVault(file) {
  if (typeof file !== 'string' || file.length === 0) {
    throw new VaultError('vault file path is required', 'EVAULT');
  }
  const absolute = path.resolve(file);
  let text;
  try {
    text = fs.readFileSync(absolute, 'utf8');
  } catch (err) {
    throw new VaultError(`cannot read vault file ${absolute}: ${err.code || err.message}`, 'EVAULT');
  }

  let document;
  try {
    document = JSON.parse(text);
  } catch (err) {
    throw new VaultError(`vault file ${absolute} is not valid JSON`, 'EVAULT');
  }
  if (document === null || typeof document !== 'object' || Array.isArray(document)) {
    throw new VaultError('vault document must be an object', 'EVAULT');
  }

  const baseDir = path.dirname(absolute);
  const credentials = new Map();
  if (document.credentials !== undefined) {
    if (document.credentials === null || typeof document.credentials !== 'object' || Array.isArray(document.credentials)) {
      throw new VaultError('credentials must be an object keyed by key id', 'EVAULTCRED');
    }
    for (const [keyId, raw] of Object.entries(document.credentials)) {
      credentials.set(keyId, normalizeCredential(keyId, raw, baseDir));
    }
  }
  if (credentials.size === 0) {
    throw new VaultError('vault contains no credentials', 'EVAULT');
  }

  if (!Array.isArray(document.rules)) {
    throw new VaultError('rules must be an array', 'EVAULTRULE');
  }
  const seenHosts = new Set();
  const rules = document.rules.map((raw) => normalizeRule(raw, credentials, seenHosts));

  return { file: absolute, credentials, rules };
}

function getCredential(vault, keyId) {
  const credential = vault.credentials.get(keyId);
  if (credential === undefined) {
    throw new VaultError(`unknown credential: ${keyId}`, 'EVAULTCRED');
  }
  return credential;
}

/**
 * Returns the header pairs that must be written on the wire for a credential.
 * This is the single place that turns vault material into request headers.
 */
function injectionHeaders(credential) {
  const headers = new Map();
  const value = credential.scheme === null ? credential.secret : `${credential.scheme} ${credential.secret}`;
  headers.set(credential.header.toLowerCase(), { name: credential.header, value });
  for (const [lower, entry] of credential.headers) {
    if (!headers.has(lower)) headers.set(lower, entry);
  }
  return headers;
}

function clientMaterial(credential) {
  if (credential === null || credential.clientCert === null) return null;
  const { certFile, keyFile, passphrase } = credential.clientCert;
  let cert;
  let key;
  try {
    cert = fs.readFileSync(certFile);
    key = fs.readFileSync(keyFile);
  } catch (err) {
    throw new VaultError('cannot read client certificate material', 'EVAULTCRED');
  }
  return { cert, key, passphrase };
}

module.exports = {
  VaultError,
  loadVault,
  getCredential,
  injectionHeaders,
  clientMaterial,
  normalizeHost,
};

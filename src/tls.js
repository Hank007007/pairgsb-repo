'use strict';

const fs = require('node:fs');
const https = require('node:https');
const tls = require('node:tls');

const { clientMaterial } = require('./vault');

let cachedCa = null;
let cachedCaFile = null;

function loadTrustAnchors(caFile) {
  const file = caFile !== undefined && caFile !== null ? caFile : process.env.EGRESS_CA_FILE;
  if (typeof file !== 'string' || file.length === 0) return null;
  if (cachedCaFile === file && cachedCa !== null) return cachedCa;
  const extra = fs.readFileSync(file);
  cachedCa = [...tls.rootCertificates, extra.toString('utf8')];
  cachedCaFile = file;
  return cachedCa;
}

/**
 * Connection parameters for one upstream TLS hop. `credential` is the vault
 * entry selected for that hop; its client certificate, when present, is the
 * only material offered to the peer.
 */
function agentOptions(options) {
  const host = options.host;
  const params = {
    keepAlive: false,
    servername: host,
    rejectUnauthorized: true,
  };

  const ca = loadTrustAnchors(options.caFile);
  if (ca !== null) params.ca = ca;

  if (options.credential) {
    const material = clientMaterial(options.credential);
    if (material !== null) {
      params.cert = material.cert;
      params.key = material.key;
      if (material.passphrase !== undefined) params.passphrase = material.passphrase;
    }
  }
  return params;
}

function createUpstreamAgent(options) {
  return new https.Agent(agentOptions(options));
}

function resetTrustCache() {
  cachedCa = null;
  cachedCaFile = null;
}

module.exports = {
  agentOptions,
  createUpstreamAgent,
  resetTrustCache,
};

'use strict';

const http = require('node:http');
const https = require('node:https');
const net = require('node:net');
const tls = require('node:tls');

const { loadVault, getCredential, injectionHeaders } = require('./vault');
const { canonicalHost, selectCredential, matchesNoProxy } = require('./routing');
const { agentOptions } = require('./tls');
const { isRedirect, preservesBody, follow } = require('./redirect');
const { createAuditLog } = require('./audit');

const MAX_BODY_BYTES = 8 * 1024 * 1024;
const DEFAULT_MAX_REDIRECTS = 5;

class ProxyError extends Error {
  constructor(code, message) {
    super(message || code);
    this.name = 'ProxyError';
    this.code = code;
  }
}

function parseIntEnv(value, fallback) {
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) && n >= 0 ? n : fallback;
}

function envValue(env, name) {
  const value = env[name];
  return typeof value === 'string' && value.length > 0 ? value : null;
}

/**
 * Picks the upstream proxy for a hop, if one is configured.
 */
function selectProxy(hop, env) {
  const schemeVar = hop.protocol === 'https:' ? 'HTTPS_PROXY' : 'HTTP_PROXY';
  const forScheme = envValue(env, schemeVar);
  if (forScheme !== null) return new URL(forScheme);

  const catchAll = envValue(env, 'ALL_PROXY');
  if (catchAll === null) return null;
  if (matchesNoProxy(hop.host, hop.port, env)) return null;
  return new URL(catchAll);
}

function headerEntriesToMap(rawHeaders) {
  const map = new Map();
  const names = new Map();
  for (let i = 0; i < rawHeaders.length; i += 2) {
    const name = rawHeaders[i];
    const value = rawHeaders[i + 1];
    const lower = name.toLowerCase();
    if (names.has(lower)) {
      const existing = map.get(lower);
      map.set(lower, { name: existing.name, value: `${existing.value}, ${value}` });
    } else {
      names.set(lower, name);
      map.set(lower, { name, value });
    }
  }
  return map;
}

function applyCredential(headers, credential) {
  if (credential === null) return headers;
  for (const [lower, entry] of injectionHeaders(credential)) {
    headers.set(lower, entry);
  }
  return headers;
}

function headersToObject(headers) {
  const out = {};
  for (const [lower, entry] of headers) {
    out[lower] = entry.value;
  }
  return out;
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(new ProxyError('EBADREQUEST', 'request body too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

function targetFromRequest(req) {
  const raw = typeof req.url === 'string' && req.url.length > 0 ? req.url : '/';
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) {
    let parsed;
    try {
      parsed = new URL(raw);
    } catch (err) {
      throw new ProxyError('EBADREQUEST', 'request target is not a valid absolute URI');
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      throw new ProxyError('EUNSUPPORTEDSCHEME', `unsupported scheme: ${parsed.protocol}`);
    }
    return parsed;
  }
  const hostHeader = req.headers.host;
  if (typeof hostHeader !== 'string' || hostHeader.length === 0) {
    throw new ProxyError('EBADREQUEST', 'request has no absolute URI and no Host header');
  }
  try {
    return new URL(`http://${hostHeader}${raw}`);
  } catch (err) {
    throw new ProxyError('EBADREQUEST', 'request target is not a valid URI');
  }
}

function sendDirect(hop, headers, body, opts, credential) {
  return new Promise((resolve, reject) => {
    const options = {
      method: hop.method,
      headers: headersToObject(headers),
      host: hop.host,
      port: hop.port,
      path: hop.path,
      timeout: opts.timeoutMs,
    };

    let transport = http;
    if (hop.protocol === 'https:') {
      transport = https;
      Object.assign(options, agentOptions({ host: hop.host, caFile: opts.caFile, credential }));
    }

    const req = transport.request(options, (res) => {
      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => {
        resolve({
          statusCode: res.statusCode,
          headers: res.headers,
          rawHeaders: res.rawHeaders,
          body: Buffer.concat(chunks),
        });
      });
      res.on('error', reject);
    });

    req.on('timeout', () => {
      req.destroy(new ProxyError('ETIMEOUT', 'upstream timed out'));
    });
    req.on('error', reject);
    if (body !== null && body.length > 0) req.write(body);
    req.end();
  });
}

function sendViaProxy(hop, headers, body, opts, credential, proxyUrl) {
  if (hop.protocol === 'https:') {
    return sendHttpsThroughProxy(hop, headers, body, opts, credential, proxyUrl);
  }
  return new Promise((resolve, reject) => {
    const proxyHeaders = headersToObject(headers);
    proxyHeaders.host = hop.host;
    const target = `${hop.protocol}//${hop.host}:${hop.port}${hop.path}`;
    const req = http.request(
      {
        method: hop.method,
        host: proxyUrl.hostname,
        port: proxyUrl.port === '' ? 80 : Number(proxyUrl.port),
        path: target,
        headers: proxyHeaders,
        timeout: opts.timeoutMs,
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          resolve({
            statusCode: res.statusCode,
            headers: res.headers,
            rawHeaders: res.rawHeaders,
            body: Buffer.concat(chunks),
          });
        });
        res.on('error', reject);
      }
    );
    req.on('timeout', () => req.destroy(new ProxyError('ETIMEOUT', 'upstream timed out')));
    req.on('error', reject);
    if (body !== null && body.length > 0) req.write(body);
    req.end();
  });
}

function sendHttpsThroughProxy(hop, headers, body, opts, credential, proxyUrl) {
  return new Promise((resolve, reject) => {
    const proxyPort = proxyUrl.port === '' ? 80 : Number(proxyUrl.port);
    const socket = net.connect(proxyPort, proxyUrl.hostname);
    let settled = false;
    const fail = (err) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      reject(err);
    };

    socket.once('error', fail);
    socket.once('connect', () => {
      socket.write(`CONNECT ${hop.host}:${hop.port} HTTP/1.1\r\nHost: ${hop.host}:${hop.port}\r\n\r\n`);
    });

    let handshake = '';
    const onData = (chunk) => {
      handshake += chunk.toString('latin1');
      const end = handshake.indexOf('\r\n\r\n');
      if (end === -1) return;
      socket.removeListener('data', onData);
      const statusLine = handshake.slice(0, handshake.indexOf('\r\n'));
      if (!/ 200 /.test(statusLine)) {
        fail(new ProxyError('EUPSTREAM', `proxy refused CONNECT: ${statusLine}`));
        return;
      }
      const tlsOptions = Object.assign(
        agentOptions({ host: hop.host, caFile: opts.caFile, credential }),
        { socket, host: hop.host }
      );
      const secure = tls.connect(tlsOptions, () => {
        const req = https.request(
          {
            method: hop.method,
            headers: headersToObject(headers),
            host: hop.host,
            port: hop.port,
            path: hop.path,
            createConnection: () => secure,
            timeout: opts.timeoutMs,
          },
          (res) => {
            const chunks = [];
            res.on('data', (c) => chunks.push(c));
            res.on('end', () => {
              settled = true;
              resolve({
                statusCode: res.statusCode,
                headers: res.headers,
                rawHeaders: res.rawHeaders,
                body: Buffer.concat(chunks),
              });
            });
            res.on('error', fail);
          }
        );
        req.on('timeout', () => req.destroy(new ProxyError('ETIMEOUT', 'upstream timed out')));
        req.on('error', fail);
        if (body !== null && body.length > 0) req.write(body);
        req.end();
      });
      secure.once('error', fail);
    };
    socket.on('data', onData);
  });
}

function sendHop(hop, headers, body, opts, credential) {
  const proxyUrl = selectProxy(hop, opts.env);
  if (proxyUrl !== null) {
    return sendViaProxy(hop, headers, body, opts, credential, proxyUrl);
  }
  return sendDirect(hop, headers, body, opts, credential);
}

function writeError(res, code) {
  if (res.headersSent) {
    res.destroy();
    return;
  }
  const payload = JSON.stringify({ error: { code } });
  res.writeHead(502, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(payload),
  });
  res.end(payload);
}

const TLS_ERROR_CODES = new Set([
  'DEPTH_ZERO_SELF_SIGNED_CERT',
  'SELF_SIGNED_CERT_IN_CHAIN',
  'UNABLE_TO_VERIFY_LEAF_SIGNATURE',
  'UNABLE_TO_GET_ISSUER_CERT',
  'UNABLE_TO_GET_ISSUER_CERT_LOCALLY',
  'CERT_HAS_EXPIRED',
  'CERT_NOT_YET_VALID',
  'CERT_REVOKED',
  'CERT_UNTRUSTED',
  'ERR_TLS_CERT_ALTNAME_INVALID',
  'HOSTNAME_MISMATCH',
]);

/**
 * Collapses whatever the transport layer threw into one of the error codes the
 * caller is allowed to see (§8.1). Anything unrecognised is an upstream fault.
 */
function callerErrorCode(err) {
  if (!err || typeof err.code !== 'string') return 'EUPSTREAM';
  const code = err.code;
  if (code === 'ETIMEOUT') return 'ETIMEOUT';
  if (code === 'ELOOP' || code === 'EBADREDIRECT' || code === 'EBADREQUEST') return code;
  if (code === 'ENOTFOUND' || code === 'EHOSTUNREACH' || code === 'ENETUNREACH') return 'ENOROUTE';
  if (TLS_ERROR_CODES.has(code) || code.startsWith('ERR_TLS') || code.startsWith('ERR_SSL')) return 'EUPSTREAMTLS';
  return 'EUPSTREAM';
}

function relay(res, upstream) {
  const headers = {};
  for (let i = 0; i < upstream.rawHeaders.length; i += 2) {
    const name = upstream.rawHeaders[i];
    const lower = name.toLowerCase();
    if (lower === 'connection' || lower === 'transfer-encoding' || lower === 'keep-alive') continue;
    if (/^proxy-/.test(lower)) continue;
    headers[name] = upstream.rawHeaders[i + 1];
  }
  res.writeHead(upstream.statusCode, headers);
  res.end(upstream.body);
}

function createProxy(options) {
  const env = options.env || process.env;
  const vault = options.vault || loadVault(options.vaultFile || env.EGRESS_VAULT_FILE);
  const audit = options.audit || createAuditLog({ file: options.auditFile || env.EGRESS_AUDIT_FILE });
  const maxRedirects = parseIntEnv(options.maxRedirects || env.EGRESS_MAX_REDIRECTS, DEFAULT_MAX_REDIRECTS);
  const opts = {
    env,
    caFile: options.caFile || env.EGRESS_CA_FILE,
    timeoutMs: parseIntEnv(options.timeoutMs || env.EGRESS_TIMEOUT_MS, 30000),
  };

  async function handle(req, res) {
    let target;
    try {
      target = targetFromRequest(req);
    } catch (err) {
      writeError(res, err.code || 'EBADREQUEST');
      return;
    }
    if (target.protocol !== 'http:' && target.protocol !== 'https:') {
      writeError(res, 'EUNSUPPORTEDSCHEME');
      return;
    }

    let body;
    try {
      body = await readBody(req);
    } catch (err) {
      writeError(res, err.code || 'EBADREQUEST');
      return;
    }

    let hop = {
      url: target.href,
      host: canonicalHost(target.hostname),
      port: target.port === '' ? (target.protocol === 'https:' ? 443 : 80) : Number(target.port),
      protocol: target.protocol,
      path: `${target.pathname}${target.search}`,
      method: req.method || 'GET',
      headers: headerEntriesToMap(req.rawHeaders),
      body: body.length > 0 ? body : null,
      hop: 0,
    };
    hop.headers.delete('host');
    hop.headers.delete('proxy-connection');

    const chain = [];
    for (;;) {
      const started = Date.now();
      const rule = selectCredential(vault.rules, hop.host);
      let credential = null;
      if (rule !== null) {
        credential = getCredential(vault, rule.credential);
        applyCredential(hop.headers, credential);
      }

      let upstream = null;
      let error = null;
      try {
        upstream = await sendHop(hop, hop.headers, hop.body, opts, credential);
      } catch (err) {
        error = err;
      }

      audit.logRequest({
        ts: new Date(started),
        method: hop.method,
        url: hop.url,
        host: hop.host,
        path: hop.path,
        status: upstream === null ? null : upstream.statusCode,
        durationMs: Date.now() - started,
        keyId: credential === null ? null : credential.keyId,
        headers: hop.headers,
        error,
      });

      if (error !== null) {
        writeError(res, callerErrorCode(error));
        return;
      }

      const status = upstream.statusCode;
      if (isRedirect(status) && typeof upstream.headers.location === 'string') {
        if (chain.length >= maxRedirects) {
          writeError(res, 'ELOOP');
          return;
        }
        chain.push(hop.url);
        try {
          hop = follow(hop, upstream);
        } catch (err) {
          writeError(res, 'EBADREDIRECT');
          return;
        }
        if (!preservesBody(status)) hop.body = null;
        continue;
      }

      relay(res, upstream);
      return;
    }
  }

  const server = http.createServer((req, res) => {
    handle(req, res).catch(() => writeError(res, 'EUPSTREAM'));
  });

  return {
    server,
    handle,
    vault,
    audit,
    close() {
      audit.close();
      return new Promise((resolve) => server.close(resolve));
    },
  };
}

module.exports = {
  createProxy,
  ProxyError,
  selectProxy,
  applyCredential,
  targetFromRequest,
};

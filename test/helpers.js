'use strict';

const fs = require('node:fs');
const http = require('node:http');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');

const FIXTURES = path.join(__dirname, 'fixtures');

const readFixture = (name) => fs.readFileSync(path.join(FIXTURES, name));
const fixturePath = (name) => path.join(FIXTURES, name);

/**
 * Starts an HTTP upstream that records every request it sees and answers with
 * whatever `respond` returns for that request.
 */
async function startHttpUpstream(respond) {
  const seen = [];
  const server = http.createServer((req, res) => {
    const chunks = [];
    req.on('data', (chunk) => chunks.push(chunk));
    req.on('end', () => {
      const record = {
        method: req.method,
        url: req.url,
        headers: req.headers,
        body: Buffer.concat(chunks).toString('utf8'),
      };
      seen.push(record);
      const reply = respond(record, seen.length);
      res.writeHead(reply.status, reply.headers || { 'content-type': 'text/plain' });
      res.end(reply.body === undefined ? 'ok' : reply.body);
    });
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  return {
    port,
    seen,
    url: `http://127.0.0.1:${port}`,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}

async function startHttpsUpstream(respond) {
  const seen = [];
  const server = https.createServer(
    {
      cert: readFixture('server.pem'),
      key: readFixture('server-key.pem'),
    },
    (req, res) => {
      const chunks = [];
      req.on('data', (chunk) => chunks.push(chunk));
      req.on('end', () => {
        const record = {
          method: req.method,
          url: req.url,
          headers: req.headers,
          body: Buffer.concat(chunks).toString('utf8'),
        };
        seen.push(record);
        const reply = respond(record, seen.length);
        res.writeHead(reply.status, reply.headers || { 'content-type': 'text/plain' });
        res.end(reply.body === undefined ? 'ok' : reply.body);
      });
    }
  );
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  return {
    port,
    seen,
    url: `https://localhost:${port}`,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}

/**
 * Minimal forwarding proxy used as the "upstream proxy" in tests. It records
 * the absolute-form request target and relays the request to the origin.
 */
async function startForwardProxy() {
  const seen = [];
  const server = http.createServer((req, res) => {
    seen.push({ method: req.method, url: req.url, headers: req.headers });
    let target;
    try {
      target = new URL(req.url);
    } catch (err) {
      res.writeHead(400).end('bad target');
      return;
    }
    const upstream = http.request(
      {
        method: req.method,
        host: target.hostname,
        port: target.port === '' ? 80 : Number(target.port),
        path: `${target.pathname}${target.search}`,
        headers: req.headers,
      },
      (upstreamRes) => {
        res.writeHead(upstreamRes.statusCode, upstreamRes.headers);
        upstreamRes.pipe(res);
      }
    );
    upstream.on('error', () => {
      res.writeHead(502).end('upstream error');
    });
    req.pipe(upstream);
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  return {
    port,
    seen,
    url: `http://127.0.0.1:${port}`,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}

function makeTempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'egressvault-test-'));
}

function readAudit(file) {
  if (!fs.existsSync(file)) return [];
  return fs
    .readFileSync(file, 'utf8')
    .split('\n')
    .filter((line) => line.trim().length > 0)
    .map((line) => JSON.parse(line));
}

/**
 * Sends a request through the proxy using absolute-form, the way a business
 * process configured with HTTP_PROXY would.
 */
function requestThroughProxy(proxyPort, targetUrl, options = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        host: '127.0.0.1',
        port: proxyPort,
        method: options.method || 'GET',
        path: targetUrl,
        headers: options.headers || {},
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () =>
          resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks).toString('utf8') })
        );
      }
    );
    req.on('error', reject);
    if (options.body) req.write(options.body);
    req.end();
  });
}

async function listen(proxy) {
  await new Promise((resolve) => proxy.server.listen(0, '127.0.0.1', resolve));
  return proxy.server.address().port;
}

module.exports = {
  FIXTURES,
  fixturePath,
  readFixture,
  startHttpUpstream,
  startHttpsUpstream,
  startForwardProxy,
  makeTempDir,
  readAudit,
  requestThroughProxy,
  listen,
};

import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const defaultBundle = fileURLToPath(new URL('./bundle.sample.json', import.meta.url));
const DEFAULT_PORT = 8200;
const DEFAULT_TTL_SECONDS = 300;

function fail(message) {
  process.stderr.write(`mock-broker: ${message}\n`);
  process.exit(1);
}

function parseArgs(argv) {
  const options = {
    port: Number(process.env.MOCK_BROKER_PORT ?? DEFAULT_PORT),
    bundlePath: process.env.MOCK_BUNDLE ?? defaultBundle,
    service: process.env.MOCK_SERVICE ?? '',
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--port') options.port = Number(argv[++i]);
    else if (arg === '--bundle') options.bundlePath = argv[++i] ?? '';
    else if (arg === '--service') options.service = argv[++i] ?? '';
    else fail(`unknown argument ${arg}`);
  }

  if (!options.bundlePath) {
    fail('a bundle file is required: --bundle <path>');
  }
  if (!Number.isInteger(options.port) || options.port <= 0) {
    fail('--port must be a positive integer');
  }

  return options;
}

function readBundle(bundlePath) {
  let parsed;
  try {
    parsed = JSON.parse(fs.readFileSync(bundlePath, 'utf8'));
  } catch (err) {
    fail(`cannot read bundle ${bundlePath}: ${err.message}`);
  }

  if (parsed === null || typeof parsed !== 'object' || typeof parsed.secrets !== 'object' || parsed.secrets === null) {
    fail(`bundle ${bundlePath} must be an object with a secrets map`);
  }

  return {
    ttlSeconds: Number.isFinite(parsed.ttlSeconds) ? parsed.ttlSeconds : DEFAULT_TTL_SECONDS,
    secrets: parsed.secrets,
  };
}

function sendJson(response, status, payload) {
  const body = JSON.stringify(payload);
  response.writeHead(status, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(body),
  });
  response.end(body);
}

function main() {
  const options = parseArgs(process.argv.slice(2));
  const bundlePath = path.resolve(options.bundlePath);
  let leaseCounter = 0;
  let currentLeaseId = '';

  const server = http.createServer((request, response) => {
    const url = new URL(request.url, 'http://127.0.0.1');
    process.stderr.write(`${request.method} ${url.pathname}\n`);

    const issue = url.pathname.match(/^\/v1\/secrets\/([^/]+)$/);
    if (request.method === 'GET' && issue) {
      const service = decodeURIComponent(issue[1]);
      if (options.service && service !== options.service) {
        sendJson(response, 404, { error: 'unknown service' });
        return;
      }
      const bundle = readBundle(bundlePath);
      leaseCounter += 1;
      currentLeaseId = `lease-${leaseCounter}`;
      sendJson(response, 200, {
        leaseId: currentLeaseId,
        ttlSeconds: bundle.ttlSeconds,
        secrets: bundle.secrets,
      });
      return;
    }

    const renewal = url.pathname.match(/^\/v1\/leases\/([^/]+)\/renew$/);
    if (request.method === 'POST' && renewal) {
      const leaseId = decodeURIComponent(renewal[1]);
      if (leaseId !== currentLeaseId) {
        sendJson(response, 404, { error: 'unknown lease' });
        return;
      }
      const bundle = readBundle(bundlePath);
      leaseCounter += 1;
      currentLeaseId = `lease-${leaseCounter}`;
      sendJson(response, 200, {
        leaseId: currentLeaseId,
        ttlSeconds: bundle.ttlSeconds,
        secrets: bundle.secrets,
      });
      return;
    }

    sendJson(response, 404, { error: 'not found' });
  });

  server.listen(options.port, '127.0.0.1', () => {
    process.stderr.write(
      `mock-broker listening on http://127.0.0.1:${options.port} (bundle: ${bundlePath})\n`,
    );
  });

  const shutdown = () => server.close(() => process.exit(0));
  process.once('SIGINT', shutdown);
  process.once('SIGTERM', shutdown);
}

main();

#!/usr/bin/env node
'use strict';

// envseald — secret injection sidecar for CI runners.

const fs = require('node:fs');
const path = require('node:path');

const { createAppFromFiles } = require('../lib/app');

const USAGE = `usage: envseald --socket <path> --leases-dir <dir> --policy <file> \\
                --pipelines <dir> --store <file> --store-key <file>`;

function parseArgs(argv) {
  const out = {};
  const known = new Set([
    'socket',
    'leases-dir',
    'policy',
    'pipelines',
    'store',
    'store-key',
  ]);

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith('--')) continue;
    const name = arg.slice(2);
    if (!known.has(name)) continue;
    const value = argv[i + 1];
    if (value === undefined || value.startsWith('--')) continue;
    out[name] = value;
    i += 1;
  }

  return out;
}

function fail(message) {
  process.stderr.write(`envseald: ${message}\n`);
  process.exit(2);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const required = ['socket', 'leases-dir', 'policy', 'pipelines', 'store', 'store-key'];
  const missing = required.filter((name) => !args[name]);
  if (missing.length > 0) {
    process.stderr.write(USAGE + '\n');
    fail(`missing required option(s): ${missing.map((n) => '--' + n).join(', ')}`);
  }

  const leasesDir = path.resolve(args['leases-dir']);
  fs.mkdirSync(leasesDir, { recursive: true });

  let app;
  try {
    app = createAppFromFiles({
      policyPath: path.resolve(args.policy),
      pipelinesDir: path.resolve(args.pipelines),
      storePath: path.resolve(args.store),
      storeKeyPath: path.resolve(args['store-key']),
      leasesDir,
    });
  } catch (error) {
    fail(`failed to load configuration: ${error.message}`);
  }

  const rawSocket = args.socket;
  // On Windows a filesystem path cannot be listened on; map it to a named pipe
  // under the same name so the documented invocation works on both platforms.
  const isNamedPipe = process.platform === 'win32' && rawSocket.startsWith('\\\\');
  const socketPath = isNamedPipe
    ? rawSocket
    : process.platform === 'win32'
      ? '\\\\.\\pipe\\' + path.basename(rawSocket)
      : path.resolve(rawSocket);
  if (process.platform !== 'win32') fs.mkdirSync(path.dirname(socketPath), { recursive: true });

  const sweepTimer = setInterval(() => {
    app.leases.sweep();
  }, 1000);
  sweepTimer.unref();

  app.server.listen(socketPath, () => {
    process.stdout.write(`envseald listening on ${socketPath}\n`);
  });

  app.server.on('error', (error) => {
    fail(`cannot listen on ${socketPath}: ${error.message}`);
  });

  const shutdown = () => {
    clearInterval(sweepTimer);
    app.server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 2000).unref();
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

main();

#!/usr/bin/env node
'use strict';

const path = require('node:path');

const { shipOnce } = require('../src/forwarder/client');
const { createCollector } = require('../src/collector/server');
const { loadKeyring, readSecret } = require('../src/config');

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (token.startsWith('--')) {
      const [flag, inline] = token.slice(2).split('=');
      if (inline !== undefined) {
        out[flag] = inline;
      } else if (argv[i + 1] !== undefined && !argv[i + 1].startsWith('--')) {
        out[flag] = argv[i + 1];
        i += 1;
      } else {
        out[flag] = true;
      }
    } else {
      out._.push(token);
    }
  }
  return out;
}

function usage() {
  return [
    'usage:',
    '  spoolctl ship  --spool <dir> --state <dir> --collector <url> --key-id <id> --secret-file <path> [--limit N]',
    '  spoolctl serve --store <dir> --keyring <path> [--host <addr>] [--port N]',
  ].join('\n');
}

async function runShip(args) {
  const required = ['spool', 'state', 'collector', 'key-id', 'secret-file'];
  for (const flag of required) {
    if (!args[flag]) {
      process.stderr.write(`spoolctl ship: missing --${flag}\n${usage()}\n`);
      return 2;
    }
  }
  const secret = readSecret(args['secret-file']);
  const results = await shipOnce({
    spoolDir: path.resolve(args.spool),
    stateDir: path.resolve(args.state),
    collectorUrl: args.collector,
    keyId: args['key-id'],
    secret,
    limit: args.limit ? Number.parseInt(args.limit, 10) : undefined,
  });
  if (results.length === 0) {
    process.stdout.write('spoolctl ship: nothing to send\n');
    return 0;
  }
  for (const result of results) {
    process.stdout.write(
      `spoolctl ship: origin=${result.origin} epoch=${result.epoch} firstSeq=${result.firstSeq} count=${result.count} ` +
        `watermark=${result.response && result.response.watermark}\n`,
    );
  }
  return 0;
}

async function runServe(args) {
  if (!args.store || !args.keyring) {
    process.stderr.write(`spoolctl serve: missing --store or --keyring\n${usage()}\n`);
    return 2;
  }
  const keyring = loadKeyring(args.keyring);
  const server = createCollector({ storeDir: path.resolve(args.store), keyring });
  const port = args.port ? Number.parseInt(args.port, 10) : 8787;
  const host = args.host || '127.0.0.1';

  await new Promise((resolve) => server.listen(port, host, resolve));
  const address = server.address();
  process.stdout.write(`spoolctl serve: listening on http://${host}:${address.port}\n`);

  const shutdown = () => server.close(() => process.exit(0));
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
  return null;
}

async function main(argv) {
  const args = parseArgs(argv);
  const command = args._[0];
  if (command === 'ship') return runShip(args);
  if (command === 'serve') return runServe(args);
  process.stderr.write(`${usage()}\n`);
  return 2;
}

if (require.main === module) {
  main(process.argv.slice(2))
    .then((code) => {
      if (typeof code === 'number') process.exitCode = code;
    })
    .catch((err) => {
      process.stderr.write(`spoolctl: ${err.message}\n`);
      process.exitCode = 1;
    });
}

module.exports = { main, parseArgs };

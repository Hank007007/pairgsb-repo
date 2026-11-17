#!/usr/bin/env node
import path from 'node:path';
import os from 'node:os';
import fs from 'node:fs';
import { createService, IssueError } from '../src/index.js';
import { runDemo } from '../src/demo.js';

const USAGE = `keyladder <command> [options]

Commands:
  issue --owner <id> --scopes <a,b> [--env <env>]   issue a new key
  verify <key> [--env <env>]                        verify a presented key
  rotate <id8>                                      rotate a key
  revoke <id8>                                      revoke a key
  list                                              list stored records
  dump                                              print the raw data file
  demo                                              run the lifecycle demo

Global options:
  --data <file>     path to the data file (default: data/keys.json)
  --log-level <l>   debug | info | warn | error (default: info)
`;

function parseArgv(argv) {
  const positional = [];
  const flags = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token.startsWith('--')) {
      const name = token.slice(2);
      const next = argv[index + 1];
      if (next === undefined || next.startsWith('--')) {
        flags[name] = true;
      } else {
        flags[name] = next;
        index += 1;
      }
    } else {
      positional.push(token);
    }
  }
  return { positional, flags };
}

function output(value) {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

async function main() {
  const { positional, flags } = parseArgv(process.argv.slice(2));
  const command = positional[0];

  if (!command || command === 'help' || flags.help) {
    process.stdout.write(USAGE);
    return 0;
  }

  if (command === 'demo') {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'keyladder-demo-'));
    const dataFile = path.join(directory, 'keys.json');
    try {
      runDemo({ dataFile });
    } finally {
      fs.rmSync(directory, { recursive: true, force: true });
    }
    return 0;
  }

  const service = createService({
    dataFile: flags.data ? path.resolve(flags.data) : undefined,
    logLevel: flags['log-level'],
  });

  switch (command) {
    case 'issue': {
      output(
        service.issue({
          ownerId: flags.owner,
          scopes: flags.scopes ? String(flags.scopes).split(',').filter(Boolean) : [],
          env: flags.env ?? 'prod',
        }),
      );
      return 0;
    }
    case 'verify': {
      const result = service.verify(positional[1], { env: flags.env });
      output(result);
      return result.ok ? 0 : 1;
    }
    case 'rotate': {
      output(service.rotate(positional[1]));
      return 0;
    }
    case 'revoke': {
      output(service.revoke(positional[1]));
      return 0;
    }
    case 'list': {
      output(service.list());
      return 0;
    }
    case 'dump': {
      output(service.store.all());
      return 0;
    }
    default: {
      process.stderr.write(`unknown command: ${command}\n\n${USAGE}`);
      return 2;
    }
  }
}

try {
  process.exitCode = await main();
} catch (error) {
  if (error instanceof IssueError) {
    process.stderr.write(`${error.code}: ${error.message}\n`);
    process.exitCode = 1;
  } else {
    throw error;
  }
}

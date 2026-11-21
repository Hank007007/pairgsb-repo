#!/usr/bin/env node
import { readFileSync, writeFileSync } from 'node:fs';
import { handleInvite } from '../src/http/invite.js';

function usage() {
  process.stderr.write(
    [
      'Usage: invitefence <request.json> [--ics out.ics] [--mime out.eml]',
      '',
      '  <request.json>   path to a JSON invite request',
      '  --ics <path>     write the generated .ics document to <path>',
      '  --mime <path>    write the generated MIME message to <path>',
      '  --json           print the full handler result as JSON',
      '',
    ].join('\n') + '\n',
  );
}

function main(argv) {
  const args = argv.slice(2);
  if (args.length === 0 || args[0] === '-h' || args[0] === '--help') {
    usage();
    process.exit(args.length === 0 ? 1 : 0);
  }

  const inputPath = args[0];
  const options = { ics: null, mime: null, json: false };
  for (let i = 1; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--ics') options.ics = args[++i];
    else if (arg === '--mime') options.mime = args[++i];
    else if (arg === '--json') options.json = true;
    else {
      process.stderr.write(`Unknown option: ${arg}\n`);
      usage();
      process.exit(1);
    }
  }

  let payload;
  try {
    payload = JSON.parse(readFileSync(inputPath, 'utf8'));
  } catch (error) {
    process.stderr.write(`Cannot read request: ${error.message}\n`);
    process.exit(1);
  }

  let result;
  try {
    result = handleInvite(payload);
  } catch (error) {
    process.stderr.write(`Invite rejected: ${error.message}\n`);
    process.exit(2);
  }

  if (options.json) {
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
  } else {
    process.stdout.write(`occurrences: ${result.occurrences}\n`);
    process.stdout.write(`busyMinutes: ${result.busyMinutes}\n`);
    process.stdout.write(`boundary: ${result.boundary}\n`);
  }

  if (options.ics) {
    writeFileSync(options.ics, result.ics, 'utf8');
  }
  if (options.mime) {
    writeFileSync(options.mime, result.envelope, 'utf8');
  }
}

main(process.argv);
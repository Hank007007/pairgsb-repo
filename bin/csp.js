#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { parse_policy } from '../src/policy/parse.mjs';
import { merge_with_baseline } from '../src/policy/merge.mjs';
import { emit_headers } from '../src/http/emit.mjs';
import { BASELINE_POLICY } from '../src/platform/profile.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const POLICY_DIR = path.join(HERE, '..', 'fixtures', 'policies');

function format_policy(policy) {
  const entries = Object.entries(policy);
  if (entries.length === 0) return '  (empty)';
  return entries
    .map(([name, sources]) => `  ${name}: ${sources.length ? sources.join(' ') : '(empty)'}`)
    .join('\n');
}

async function main(argv) {
  const name = argv[2];
  if (!name) {
    process.stderr.write('usage: node bin/csp.js <policy-name> [--baseline]\n');
    process.stderr.write('reads fixtures/policies/<policy-name>\n');
    return 2;
  }

  const file = path.join(POLICY_DIR, name);
  let text;
  try {
    text = await readFile(file, 'utf8');
  } catch {
    process.stderr.write(`cannot read fixtures/policies/${name}\n`);
    return 2;
  }

  process.stdout.write(`=== ${name} ===\n`);
  process.stdout.write('tenant text:\n');
  process.stdout.write(`${text.replace(/\r?\n$/, '')}\n`);

  let parsed;
  try {
    parsed = parse_policy(text);
  } catch (error) {
    process.stderr.write(`parse error: ${error.message}\n`);
    return 1;
  }
  process.stdout.write('parsed:\n');
  process.stdout.write(`${format_policy(parsed)}\n`);

  let merged;
  try {
    merged = merge_with_baseline(parsed);
  } catch (error) {
    process.stderr.write(`merge error: ${error.message}\n`);
    return 1;
  }
  process.stdout.write('merged with baseline:\n');
  process.stdout.write(`${format_policy(merged)}\n`);

  let headers;
  try {
    headers = emit_headers(parsed);
  } catch (error) {
    process.stderr.write(`emit error: ${error.message}\n`);
    return 1;
  }
  process.stdout.write('headers:\n');
  for (const [header, value] of Object.entries(headers)) {
    process.stdout.write(`  ${header}: ${value}\n`);
  }

  if (argv.includes('--baseline')) {
    process.stdout.write('baseline:\n');
    process.stdout.write(`${format_policy(BASELINE_POLICY)}\n`);
  }

  return 0;
}

main(process.argv).then(
  (code) => {
    process.exitCode = code;
  },
  (error) => {
    process.stderr.write(`${error.stack || error.message}\n`);
    process.exitCode = 1;
  },
);

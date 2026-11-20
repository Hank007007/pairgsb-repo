#!/usr/bin/env node
// Prints each fixture's original and sanitized form side by side.

import { readdir, readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { sanitize } from '../src/index.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const fixturesDir = path.resolve(here, '..', 'fixtures');

async function main() {
  const entries = (await readdir(fixturesDir))
    .filter((name) => name.endsWith('.svg'))
    .sort();

  for (const name of entries) {
    const before = await readFile(path.join(fixturesDir, name), 'utf8');
    const after = sanitize(before);

    console.log(`=== ${name} ===`);
    console.log('--- before ---');
    console.log(before.trimEnd());
    console.log('--- after ---');
    console.log(after.trimEnd());
    console.log('');
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

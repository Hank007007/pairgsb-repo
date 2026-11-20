#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const { MultipartParser, boundaryFromContentType } = require('../src/parser');
const { MultipartError } = require('../src/errors');

const FIXTURE_DIR = path.join(__dirname, '..', 'fixtures');

function loadManifest() {
  const raw = fs.readFileSync(path.join(FIXTURE_DIR, 'manifest.json'), 'utf8');
  return JSON.parse(raw);
}

function replay(manifest, request) {
  const body = fs.readFileSync(path.join(FIXTURE_DIR, request.file));
  const contentType = request.contentType || manifest.contentType;
  const parser = new MultipartParser({
    contentType,
    boundary: boundaryFromContentType(contentType),
    limits: request.limits,
  });

  // Feed the request in small slices so the streaming path is exercised.
  const step = 7;
  try {
    for (let i = 0; i < body.length; i += step) {
      parser.write(body.slice(i, i + step));
    }
    return { parts: parser.end(), error: null };
  } catch (err) {
    if (err instanceof MultipartError) {
      return { parts: parser.parts, error: err };
    }
    throw err;
  }
}

function describe(value) {
  return value === null || value === undefined ? '-' : value;
}

function main() {
  const manifest = loadManifest();
  const wanted = process.argv.slice(2);
  const requests = wanted.length
    ? manifest.requests.filter((r) => wanted.includes(r.file) || wanted.includes(path.basename(r.file, '.http')))
    : manifest.requests;

  if (requests.length === 0) {
    process.stderr.write(`no fixture matched: ${wanted.join(' ')}\n`);
    process.exitCode = 2;
    return;
  }

  let failures = 0;

  for (const request of requests) {
    const { parts, error } = replay(manifest, request);
    process.stdout.write(`\n== ${request.file}  (${request.description})\n`);

    if (error) {
      failures += 1;
      process.stdout.write(`   rejected: ${error.status} ${error.code} - ${error.message}\n`);
      process.stdout.write(`   parts: ${parts.length}\n`);
      continue;
    }

    process.stdout.write(`   parts: ${parts.length}\n`);
    for (const [i, part] of parts.entries()) {
      process.stdout.write(
        `   [${i}] name=${describe(part.name)}` +
          ` filename=${describe(part.filename)}` +
          ` contentType=${describe(part.contentType)}` +
          ` size=${part.size}\n`
      );
    }
  }

  process.stdout.write(`\n${requests.length} fixture(s) replayed, ${failures} rejected\n`);
}

main();

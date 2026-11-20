'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');

test('bin/run.js prints a verdict for every rule and event pair', () => {
  const output = execFileSync(process.execPath, [path.join(__dirname, '..', 'bin', 'run.js')], {
    encoding: 'utf8',
  });

  const rules = fs.readdirSync(path.join(__dirname, '..', 'fixtures', 'rules')).length;
  const events = fs.readdirSync(path.join(__dirname, '..', 'fixtures', 'events')).length;

  assert.equal((output.match(/^event: /gm) || []).length, events);
  for (const line of output.split('\n').filter((line) => /^ {2}\S/.test(line))) {
    assert.match(line, /\b(hit|miss)\s*$/);
  }
  assert.equal(
    output.split('\n').filter((line) => /^ {2}\S/.test(line)).length,
    rules * events,
  );
});

test('bin/run.js reports hits and misses for the private-address rule', () => {
  const output = execFileSync(process.execPath, [path.join(__dirname, '..', 'bin', 'run.js')], {
    encoding: 'utf8',
  });
  const verdicts = output.split('\n')
    .filter((entry) => entry.includes('acme/high-from-private'))
    .map((entry) => entry.trim().split(/\s+/).at(-1));
  assert.deepEqual(verdicts.sort(), ['hit', 'miss', 'miss']);
});

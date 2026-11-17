import test from 'node:test';
import assert from 'node:assert/strict';
import { parseYaml } from '../src/yaml.mjs';

test('parses nested mappings and sequences', () => {
  const document = parseYaml(`
name: ci
jobs:
  build:
    steps:
      - name: checkout
        run: git fetch --all
      - name: test
        run: node --test
`);

  assert.equal(document.name, 'ci');
  assert.equal(document.jobs.build.steps.length, 2);
  assert.equal(document.jobs.build.steps[0].name, 'checkout');
  assert.equal(document.jobs.build.steps[1].run, 'node --test');
});

test('keeps literal block scalars intact', () => {
  const document = parseYaml(`
run: |
  echo one
  echo two
`);
  assert.equal(document.run, 'echo one\necho two\n');
});

test('reads flow sequences and scalars', () => {
  const document = parseYaml(`
branches: [main, develop]
retries: 3
enabled: true
missing: null
label: 'quoted # not a comment'
`);
  assert.deepEqual(document.branches, ['main', 'develop']);
  assert.equal(document.retries, 3);
  assert.equal(document.enabled, true);
  assert.equal(document.missing, null);
  assert.equal(document.label, 'quoted # not a comment');
});

test('strips end-of-line comments outside quotes', () => {
  const document = parseYaml(`
name: ci # pipeline name
note: "a # b"
`);
  assert.equal(document.name, 'ci');
  assert.equal(document.note, 'a # b');
});

import test from 'node:test';
import assert from 'node:assert/strict';

import { parse_policy } from '../src/policy/parse.mjs';
import { merge_with_baseline } from '../src/policy/merge.mjs';
import { BASELINE_POLICY } from '../src/platform/profile.mjs';

test('baseline directives survive an empty tenant policy', () => {
  const merged = merge_with_baseline(parse_policy(''));
  assert.deepEqual(merged['default-src'], ["'self'"]);
  assert.deepEqual(merged['report-to'], ['csp-endpoint']);
});

test('a tenant directive is added to the baseline policy', () => {
  const merged = merge_with_baseline(
    parse_policy("default-src 'self'; img-src https://img.example"),
  );
  assert.deepEqual(merged['img-src'], [
    "'self'",
    'https://cdn.platform.example',
    'https://img.example',
  ]);
  assert.deepEqual(merged['script-src'], ["'self'"]);
});

test('merging a policy that matches the baseline leaves it as it was', () => {
  const merged = merge_with_baseline(parse_policy("default-src 'self'"));
  assert.deepEqual(merged['default-src'], ["'self'"]);
});

test('merging does not mutate the inputs', () => {
  const tenant = parse_policy("default-src 'self'");
  const baselineCopy = JSON.parse(JSON.stringify(BASELINE_POLICY));
  merge_with_baseline(tenant);
  assert.deepEqual(tenant, { 'default-src': ["'self'"] });
  assert.deepEqual(JSON.parse(JSON.stringify(BASELINE_POLICY)), baselineCopy);
});

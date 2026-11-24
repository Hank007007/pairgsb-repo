import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { tokenize } from '../src/policy/tokenize.mjs';
import { parse_policy } from '../src/policy/parse.mjs';
import { merge_with_baseline } from '../src/policy/merge.mjs';
import { emit_headers, serialize } from '../src/http/emit.mjs';
import { resolve_endpoint } from '../src/report/endpoint.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const bin = path.join(here, '..', 'bin', 'csp.js');

test('tokenizer treats every documented whitespace character as a separator', () => {
  const separated = `a b\tc\u000bd\fe\nf\r g\u3000h`;
  assert.deepEqual(
    tokenize(separated).map((token) => token.value),
    ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'],
  );
});

test('semicolon ends a directive even without surrounding whitespace', () => {
  assert.deepEqual(parse_policy("script-src 'self';style-src 'none'"), {
    'script-src': ["'self'"],
    'style-src': ["'none'"],
  });
});

test('directive names canonicalize and repeated directives union sources', () => {
  const parsed = parse_policy(
    'img-src https://a.example; Script-Src \'SELF\'; script-src \'self\' https://b.example',
  );
  assert.deepEqual(parsed['img-src'], ['https://a.example']);
  assert.deepEqual(parsed['script-src'], ["'self'", 'https://b.example']);
  assert.equal(Object.keys(parsed).filter((name) => name === 'script-src').length, 1);
});

test('JSON directive name casing and values follow the same contract', () => {
  const parsed = parse_policy({
    'Script-Src': ["'SELF'", 'https://a.example'],
    'script-src': "'self' https://b.example",
  });
  assert.deepEqual(parsed['script-src'], [
    "'self'",
    'https://a.example',
    'https://b.example',
  ]);
});

for (const source of [
  'data:',
  '//evil.example/x',
  '*.',
  'https://a:*:b',
  "'nonce-@@@@'",
  'http://example.com',
  '*',
]) {
  test(`parse rejects invalid source ${source}`, () => {
    assert.throws(
      () => parse_policy(`script-src ${source}`),
      (error) => {
        assert.equal(error.name, 'PolicyError');
        assert.equal(error.details.directive, 'script-src');
        assert.equal(error.details.source, source);
        return true;
      },
    );
  });
}

test("'none' cannot coexist with another source", () => {
  assert.throws(
    () => parse_policy("default-src 'none' 'self'"),
    (error) => error.details.directive === 'default-src' && error.details.source === "'none'",
  );
  assert.throws(() => parse_policy("default-src 'none'; default-src 'self'"));
});

test("tenant 'none' does not survive next to baseline sources", () => {
  const merged = merge_with_baseline(parse_policy("default-src 'none'"));
  assert.deepEqual(merged['default-src'], ["'self'"]);

  const onlyNone = merge_with_baseline(
    { 'default-src': ["'none'"] },
    { 'default-src': ["'none'"] },
  );
  assert.deepEqual(onlyNone['default-src'], ["'none'"]);
});

test('locked directives reject widening and accept narrowing', () => {
  assert.throws(
    () => merge_with_baseline(parse_policy("object-src 'self'")),
    (error) => error.details.directive === 'object-src' && error.details.source === "'self'",
  );

  const narrowed = merge_with_baseline(parse_policy("frame-ancestors 'none'"));
  assert.deepEqual(narrowed['frame-ancestors'], ["'none'"]);

  const subset = merge_with_baseline(
    parse_policy("base-uri 'self'"),
  );
  assert.deepEqual(subset['base-uri'], ["'self'"]);
});

test('report-to policy names the fixed group while endpoint resolves separately', () => {
  const tenantUrl = 'https://reports.platform.example/csp/collect';
  const merged = merge_with_baseline(parse_policy(`default-src 'self'; report-to ${tenantUrl}`));
  assert.deepEqual(merged['report-to'], ['csp-endpoint']);

  const headers = emit_headers(merged, { reportEndpoint: tenantUrl, strictEndpoints: true });
  assert.match(headers['Content-Security-Policy'], /report-to csp-endpoint$/);
  assert.equal(
    headers['Reporting-Endpoints'],
    'csp-endpoint="https://reports.platform.example/csp/collect"',
  );
});

for (const endpoint of [
  'javascript:alert(1)',
  'http://10.0.0.7/csp/x',
  'https://tenant-b.example/csp/x',
  '/csp/collect',
]) {
  test(`endpoint ${endpoint} is rejected`, () => {
    assert.throws(() => resolve_endpoint(endpoint, { tenant: 'tenant-a' }));
  });
}

test('CR/LF are whitespace only and never survive a header', () => {
  const text =
    "default-src 'self'; img-src https://ok.example\r\nhttps://evil.example\rscript-src 'self'";
  const parsed = parse_policy(text);
  assert.deepEqual(parsed['img-src'], [
    'https://ok.example',
    'https://evil.example',
    'script-src',
    "'self'",
  ]);

  const headers = emit_headers(merge_with_baseline(parsed));
  assert.doesNotMatch(headers['Content-Security-Policy'], /[\r\n]/);
});

test('emit never copies a rejected endpoint and can fall back to default', () => {
  const headers = emit_headers(
    { 'report-to': ['javascript:alert(1)'] },
    { tenant: 'tenant-a' },
  );
  assert.equal(
    headers['Reporting-Endpoints'],
    'csp-endpoint="https://reports.platform.example/csp/collect"',
  );
  assert.doesNotMatch(headers['Reporting-Endpoints'], /javascript/);
  assert.doesNotMatch(headers['Content-Security-Policy'], /javascript/);

  assert.throws(() =>
    emit_headers(
      { 'report-to': ['javascript:alert(1)'] },
      { strictEndpoints: true },
    ),
  );
});

test('serializer uses the documented directive order and unknown directives last', () => {
  const value = serialize({
    'report-to': ['csp-endpoint'],
    'img-src': ['https://img.example'],
    'default-src': ["'self'"],
    'zzz-src': ['a'],
    'script-src': ["'self'"],
    'aaa-src': ['b'],
  });
  assert.equal(
    value,
    "default-src 'self'; script-src 'self'; img-src https://img.example; report-to csp-endpoint; aaa-src b; zzz-src a",
  );
});

test('serializer removes control characters and embedded semicolons', () => {
  const value = serialize({ 'default-src': ["foo;bar\r\nbaz"] });
  assert.equal(value, 'default-src foobarbaz');
  assert.doesNotMatch(value, /[\u0000-\u001f\u007f]/);
});

test('serializer refuses values over 4096 UTF-8 bytes', () => {
  const policy = { 'img-src': Array.from({ length: 500 }, (_, i) => `https://example${i}.test/x`) };
  assert.throws(() => serialize(policy), (error) => error.details.byteLength > 4096);
  assert.throws(() => emit_headers(policy));
});

test('CLI emits the merged baseline and rejects unsafe fixtures', () => {
  const good = spawnSync(process.execPath, [bin, 'default-self.policy'], { encoding: 'utf8' });
  assert.equal(good.status, 0, good.stderr);
  assert.match(good.stdout, /headers:[\s\S]*default-src 'self'; script-src 'self'/);

  const invalid = spawnSync(process.execPath, [bin, 'invalid-source.policy'], { encoding: 'utf8' });
  assert.equal(invalid.status, 1);
  assert.match(invalid.stderr, /parse error/);

  const widened = spawnSync(process.execPath, [bin, 'widen-object.policy'], { encoding: 'utf8' });
  assert.equal(widened.status, 1);
  assert.match(widened.stderr, /cannot be widened/);

  const oversize = spawnSync(process.execPath, [bin, 'oversize.policy'], { encoding: 'utf8' });
  assert.equal(oversize.status, 1);
  assert.match(oversize.stderr, /4096/);
});

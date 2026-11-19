'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const path = require('node:path');

const format = require('../lib/format');
const issue = require('../lib/issue');
const verify = require('../lib/verify');
const { authorizedKeysLine, makeCaKey, makeSubjectKey, makeTmpDir, removeDir } = require('../test-support/keys');

const NOW = 1_760_000_000_000;

function withCa(run) {
  const dir = makeTmpDir('verify');
  try {
    const key = makeCaKey(dir, 'certbrace-v2');
    return run(dir, key);
  } finally {
    removeDir(dir);
  }
}

function issueOne(caDir, overrides = {}) {
  const { publicKey } = makeSubjectKey();
  return issue.issueCertificate({
    request: {
      role: 'deploy',
      publicKey: authorizedKeysLine(publicKey),
      ...overrides,
    },
    caDir,
    now: NOW,
    serialFile: path.join(caDir, 'serial.seq'),
  });
}

test('a long-expired certificate is reported as expired', () => {
  withCa((caDir) => {
    const { certificate } = issueOne(caDir);
    const muchLater = NOW + 100 * 365 * 24 * 3600 * 1000;
    const window = verify.checkValidity(certificate, muchLater);

    assert.equal(window.ok, false);
    assert.equal(window.reason, 'expired');
  });
});

test('the validity check reports the moment it compared against', () => {
  withCa((caDir) => {
    const { certificate } = issueOne(caDir);
    const window = verify.checkValidity(certificate, NOW);

    assert.equal(window.now, NOW);
    assert.equal(typeof window.validAfter, 'number');
    assert.equal(typeof window.validBefore, 'number');
  });
});

test('the lifetime check reports the window length', () => {
  withCa((caDir) => {
    const { certificate } = issueOne(caDir, { ttlSeconds: 900 });
    const lifetime = verify.checkLifetime(certificate);

    assert.equal(lifetime.ok, true);
    assert.equal(lifetime.ttlSeconds, 900);
  });
});

test('the self check reports a parsed certificate and its checks', () => {
  withCa((caDir, caKey) => {
    const { text } = issueOne(caDir);
    const report = verify.selfCheck(text, caKey.publicKey, { now: NOW });

    assert.ok(report.certificate);
    assert.equal(report.checks.signature.ok, true);
    assert.equal(report.checks.lifetime.ok, true);
    assert.deepEqual(report.certificate.principals, ['deploy']);
  });
});

test('the self check rejects a certificate signed by a different CA', () => {
  withCa((caDir) => {
    const { text } = issueOne(caDir);
    const otherDir = makeTmpDir('verify-other');
    try {
      const other = makeCaKey(otherDir, 'certbrace-v9');
      const report = verify.selfCheck(text, other.publicKey, { now: NOW });

      assert.equal(report.ok, false);
      assert.equal(report.checks.signature.ok, false);
    } finally {
      removeDir(otherDir);
    }
  });
});

test('the self check reports unparseable input instead of throwing', () => {
  const dir = makeTmpDir('selfcheck');
  try {
    const key = makeCaKey(dir, 'certbrace-v2');
    const report = verify.selfCheck('not a certificate', key.publicKey);

    assert.equal(report.ok, false);
    assert.match(report.errors[0], /^parse: /);
  } finally {
    removeDir(dir);
  }
});

test('the signature algorithm name is carried in the signature field', () => {
  withCa((caDir) => {
    const { text } = issueOne(caDir);
    const parsed = format.parseCertificate(text);

    assert.equal(verify.decodeSignature(parsed).algorithm, 'ssh-ed25519');
    assert.equal(verify.decodeSignature(parsed).signature.length, 64);
  });
});

test('samePublicKey compares key material, not encoding', () => {
  const { publicKey } = makeSubjectKey();
  const pem = publicKey.export({ type: 'spki', format: 'pem' });
  const derived = crypto.createPublicKey(pem);

  assert.equal(verify.samePublicKey(publicKey, derived), true);
  assert.equal(verify.samePublicKey(publicKey, makeSubjectKey().publicKey), false);
});

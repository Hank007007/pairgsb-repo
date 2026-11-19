'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const ca = require('../lib/ca');
const format = require('../lib/format');
const issue = require('../lib/issue');
const policy = require('../lib/policy');
const verify = require('../lib/verify');
const { authorizedKeysLine, makeCaKey, makeSubjectKey, makeTmpDir, removeDir } = require('../test-support/keys');

const NOW = 1_760_000_000_000;

function withCa(run) {
  const dir = makeTmpDir('ca');
  try {
    const key = makeCaKey(dir, 'certbrace-v2');
    return run(dir, key);
  } finally {
    removeDir(dir);
  }
}

function request(overrides = {}) {
  const { publicKey } = makeSubjectKey();
  return {
    role: 'deploy',
    publicKey: authorizedKeysLine(publicKey),
    ...overrides,
  };
}

test('a deploy request issues a user certificate for the deploy principal', () => {
  withCa((caDir, caKey) => {
    const result = issue.issueCertificate({
      request: request(),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    assert.equal(result.certificate.certType, format.CERT_TYPE_USER);
    assert.deepEqual(result.certificate.principals, ['deploy']);
    assert.equal(result.certificate.validAfter, BigInt(NOW / 1000));
    assert.equal(
      result.certificate.validBefore - result.certificate.validAfter,
      BigInt(policy.defaultTtlSeconds('deploy')),
    );
    assert.equal(result.ca.kid, caKey.kid);
    assert.match(result.text, /^ssh-ed25519-cert-v01@openssh\.com /);
  });
});

test('an explicit subset of the role principals is accepted', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request({ principals: ['deploy'] }),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    assert.deepEqual(result.certificate.principals, ['deploy']);
  });
});

test('breakglass issues a root certificate', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request({ role: 'breakglass' }),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    assert.deepEqual(result.certificate.principals, ['root']);
  });
});

test('a requested ttl inside the default is honoured', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request({ ttlSeconds: 600 }),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    assert.equal(
      result.certificate.validBefore - result.certificate.validAfter,
      600n,
    );
  });
});

test('force_command is written into the certificate critical options', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request({ criticalOptions: { force_command: '/usr/bin/backup' } }),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    const parsed = format.parseCertificate(result.text);
    assert.deepEqual(format.decodeOptionList(parsed.criticalOptions), {
      force_command: '/usr/bin/backup',
    });
    assert.equal(result.certificate.keyId.startsWith('certbrace-v2:deploy:'), true);
  });
});

test('the issued certificate verifies against the CA public key', () => {
  withCa((caDir, caKey) => {
    const result = issue.issueCertificate({
      request: request(),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    const parsed = format.parseCertificate(result.text);
    const report = verify.verifySignature(parsed, caKey.publicKey);

    assert.equal(report.ok, true);
    assert.equal(report.algorithm, 'ssh-ed25519');
    assert.equal(result.selfCheck.checks.signature.ok, true);
    assert.equal(verify.samePublicKey(result.ca.publicKey, caKey.publicKey), true);
  });
});

test('a certificate signed by another CA does not verify', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request(),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    const otherDir = makeTmpDir('issue-other');
    try {
      const other = ca.loadCaKey(makeCaKey(otherDir, 'certbrace-v3').file);
      const parsed = format.parseCertificate(result.text);
      assert.equal(verify.verifySignature(parsed, other.publicKey).ok, false);
    } finally {
      removeDir(otherDir);
    }
  });
});

test('the certificate text parses back into the fields that were issued', () => {
  withCa((caDir) => {
    const result = issue.issueCertificate({
      request: request({ criticalOptions: { force_command: '/usr/bin/backup' } }),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    });

    const parsed = format.parseCertificate(result.text);
    assert.equal(parsed.keyType, format.ED25519_CERT_TYPE);
    assert.equal(parsed.certType, format.CERT_TYPE_USER);
    assert.deepEqual(parsed.principals, ['deploy']);
    assert.equal(parsed.keyId, result.certificate.keyId);
    assert.equal(parsed.validAfter, result.certificate.validAfter);
    assert.equal(parsed.validBefore, result.certificate.validBefore);
    assert.deepEqual(Buffer.from(parsed.publicKeyBlob), result.certificate.publicKey.blob);
  });
});

test('the certificate is signed by the CA key loaded from the directory', () => {
  withCa((caDir) => {
    const key = issue.issueCertificate({
      request: request(),
      caDir,
      now: NOW,
      serialFile: path.join(caDir, 'serial.seq'),
    }).certificate;

    assert.equal(key.signatureKey.blob.equals(ca.loadCaKey(path.join(caDir, 'certbrace-v2.key')).publicKeyBlob), true);
  });
});

test('unknown roles are rejected before any key is used', () => {
  withCa((caDir) => {
    assert.throws(
      () =>
        issue.issueCertificate({
          request: request({ role: 'admin' }),
          caDir,
          now: NOW,
          serialFile: path.join(caDir, 'serial.seq'),
        }),
      /unknown role: admin/,
    );
  });
});

test('a missing public key is rejected', () => {
  withCa((caDir) => {
    assert.throws(
      () =>
        issue.issueCertificate({
          request: { role: 'deploy' },
          caDir,
          now: NOW,
          serialFile: path.join(caDir, 'serial.seq'),
        }),
      /missing a public key/,
    );
  });
});

test('a non-positive ttl is rejected', () => {
  withCa((caDir) => {
    assert.throws(
      () =>
        issue.issueCertificate({
          request: request({ ttlSeconds: 0 }),
          caDir,
          now: NOW,
          serialFile: path.join(caDir, 'serial.seq'),
        }),
      /ttlSeconds must be a positive number/,
    );
  });
});

#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const ca = require('../lib/ca');
const format = require('../lib/format');
const issue = require('../lib/issue');
const verify = require('../lib/verify');
const policy = require('../lib/policy');

const USAGE = `usage: certbrace <command> [options]

commands:
  issue --request <file|-> [--ca-dir <dir>] [--serial-file <file>] [--out <file>]
        Read a signing request (JSON) and print the issued certificate.
  verify <certificate-file> [--ca <public-key-file>]
        Self-check a certificate against a CA public key.
  ca [--ca-dir <dir>]
        Show which CA key would sign the next certificate.
  roles
        Print the role table.
`;

function parseArgs(argv) {
  const positional = [];
  const options = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (token.startsWith('--')) {
      const name = token.slice(2);
      const next = argv[index + 1];
      if (next === undefined || next.startsWith('--')) {
        options[name] = true;
      } else {
        options[name] = next;
        index += 1;
      }
    } else {
      positional.push(token);
    }
  }
  return { positional, options };
}

function readRequest(source) {
  const text = source === '-' ? fs.readFileSync(0, 'utf8') : fs.readFileSync(source, 'utf8');
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new Error(`request is not valid JSON: ${error.message}`);
  }
}

function readPublicKeyFile(file) {
  return format.parsePublicKey(fs.readFileSync(file, 'utf8')).keyObject;
}

function report(result) {
  return {
    ca: { kid: result.ca.kid },
    certificate: {
      serial: result.certificate.serial.toString(),
      keyId: result.certificate.keyId,
      principals: result.certificate.principals,
      validAfter: result.certificate.validAfter.toString(),
      validBefore: result.certificate.validBefore.toString(),
      criticalOptions: result.certificate.criticalOptions,
    },
    selfCheck: {
      ok: result.selfCheck.ok,
      errors: result.selfCheck.errors,
    },
    text: result.text,
  };
}

function commandIssue(options) {
  if (!options.request || options.request === true) {
    throw new Error('issue requires --request <file|->');
  }
  const request = readRequest(options.request);
  const result = issue.issueCertificate({
    request,
    caDir: options['ca-dir'] ? path.resolve(options['ca-dir']) : undefined,
    serialFile: options['serial-file'] ? path.resolve(options['serial-file']) : undefined,
  });

  if (options.out && options.out !== true) {
    fs.writeFileSync(path.resolve(options.out), result.text, 'utf8');
  }
  process.stdout.write(`${JSON.stringify(report(result), null, 2)}\n`);
  if (!result.selfCheck.ok) {
    process.exitCode = 3;
  }
}

function commandVerify(positional, options) {
  const file = positional[0];
  if (!file) {
    throw new Error('verify requires a certificate file');
  }
  const text = fs.readFileSync(path.resolve(file), 'utf8');
  let publicKey;
  if (options.ca && options.ca !== true) {
    publicKey = readPublicKeyFile(path.resolve(options.ca));
  } else {
    publicKey = ca.selectCaKey().publicKey;
  }
  const result = verify.selfCheck(text, publicKey);
  process.stdout.write(`${JSON.stringify({
    ok: result.ok,
    errors: result.errors,
    checks: result.checks,
    certificate: result.certificate
      ? {
          keyId: result.certificate.keyId,
          principals: result.certificate.principals,
          serial: result.certificate.serial.toString(),
        }
      : null,
  }, null, 2)}\n`);
  if (!result.ok) {
    process.exitCode = 3;
  }
}

function commandCa(options) {
  const caDir = options['ca-dir'] ? path.resolve(options['ca-dir']) : undefined;
  const files = ca.listCaKeyFiles(caDir);
  const selected = ca.selectCaKey({ caDir });
  process.stdout.write(`${JSON.stringify({
    caDir: caDir || ca.DEFAULT_CA_DIR,
    files,
    selected: { kid: selected.kid, retired: selected.retired },
  }, null, 2)}\n`);
}

function commandRoles() {
  const rows = policy.ROLE_NAMES.map((role) => ({
    role,
    principals: policy.allowedPrincipals(role),
    defaultTtlSeconds: policy.defaultTtlSeconds(role),
  }));
  process.stdout.write(`${JSON.stringify(rows, null, 2)}\n`);
}

function main(argv) {
  const { positional, options } = parseArgs(argv);
  const command = positional.shift();

  switch (command) {
    case 'issue':
      commandIssue(options);
      break;
    case 'verify':
      commandVerify(positional, options);
      break;
    case 'ca':
      commandCa(options);
      break;
    case 'roles':
      commandRoles();
      break;
    case undefined:
    case 'help':
    case '--help':
    case '-h':
      process.stdout.write(USAGE);
      break;
    default:
      process.stderr.write(`unknown command: ${command}\n\n${USAGE}`);
      process.exitCode = 2;
  }
}

try {
  main(process.argv.slice(2));
} catch (error) {
  if (typeof error.toJSON === 'function') {
    process.stderr.write(`${JSON.stringify(error.toJSON(), null, 2)}\n`);
  } else {
    process.stderr.write(`${error.message}\n`);
  }
  process.exitCode = 1;
}

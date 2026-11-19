# certbrace

Internal SSH certificate issuing service. Given a subject public key and a
signing request, certbrace decides — from the request's `role` — which
principals the certificate may carry, how long it lives and which critical
options it gets, then signs it with the CA key.

- Wire format and field encoding: [`docs/sshca-spec.md`](docs/sshca-spec.md)
- System design and module boundaries: [`docs/design.md`](docs/design.md)

## Layout

```
bin/certbrace.js     CLI: issue / verify / ca / roles
lib/issue.js         request normalisation and certificate assembly
lib/format.js        OpenSSH wire format encode/decode
lib/ca.js            CA key loading, selection and signing
lib/verify.js        self-check of issued certificates
lib/policy.js        role -> principals / TTL table
ca/                  CA key material (<kid>.key + <kid>.pub)
test/                node:test suites
scripts/             development helpers
```

## Requirements

Node.js 20 or newer. No third-party dependencies. `ssh-keygen` is useful for
inspecting certificates by hand but is not needed to run the service or the
tests.

## Usage

Generate a development CA key pair and point the service at it:

```
node scripts/generate-ca.js certbrace-v2
node bin/certbrace.js ca
```

Issue a certificate and inspect it:

```
node bin/certbrace.js issue --request request.json --out cert.pub
ssh-keygen -L -f cert.pub
```

A request looks like:

```json
{
  "role": "deploy",
  "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@laptop",
  "ttlSeconds": 3600,
  "criticalOptions": { "force_command": "/usr/bin/backup" }
}
```

`node bin/certbrace.js roles` prints the role table the service enforces.

## Signing requests from code

```js
const { issueCertificate } = require('./lib/issue');

const result = issueCertificate({ request });
// result.text          authorized_keys line for the issued certificate
// result.certificate   the fields that were signed
// result.ca            key id and public key of the signing CA
// result.selfCheck     result of re-parsing and re-verifying the certificate
```

`issueCertificate` accepts `caDir`, `serialFile` and `now` for callers that
need to pin the CA directory, the serial state file or the clock.

## Tests

```
node --test
```

The suites use `node:test` and `node:assert/strict` only. CA and subject keys
are generated in-process with `crypto.generateKeyPairSync('ed25519')` and
written to temporary directories; no fixed key material is checked in.

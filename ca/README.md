# ca/ — CA key material for development

Each CA key pair is stored under its key id:

```
ca/<kid>.key     PKCS#8 PEM private key, used by certbrace to sign
ca/<kid>.pub     OpenSSH public key, loaded by sshd as TrustedUserCAKeys
```

Key ids look like `certbrace-v2`. When a CA is rotated, the old private key
stays on disk and its key id takes a `.retired` suffix, for example
`certbrace-v1.retired.key`, so that certificates issued before the rotation
can still be checked against it.

The keys committed here are throwaway development material generated with
`node scripts/generate-ca.js <kid>`. Replace them before any real deployment;
never reuse them outside local testing.

Generate a fresh pair:

```
node scripts/generate-ca.js certbrace-v2
```

List which key the service would currently use:

```
node bin/certbrace.js ca
```

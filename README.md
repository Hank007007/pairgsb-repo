# derbrace

`derbrace` is the client-certificate identity layer that sits beside the
internal mTLS gateway. The gateway forwards the DER bytes of the certificate a
peer presented; this service decodes them into a certificate structure, derives
the caller identity, and reports whether the certificate chains to a configured
anchor.

Because the bytes come from the peer, the decoder is written against X.690
directly rather than delegating parsing to a third-party ASN.1 library.

## Layout

```
derbrace/
  der.py        TLV reader, length/OID/time/integer primitives
  x509.py       certificate and extension parsing
  chain.py      chain building and structural checks
  identity.py   SAN parsing and caller identity selection
  limits.py     decode budget (element count, depth, element size)
  errors.py     typed errors
examples/
  inspect.py    inspect every certificate in fixtures/certs/
fixtures/
  certs/        peer-supplied certificate bytes, good and hostile
  pool/         intermediate certificates
  anchors/      trust anchors
docs/
  der-policy.md normative acceptance rules
tests/          pytest suite
```

## Running

```
python -m pytest -q -p no:cacheprovider
python examples/inspect.py
```

`examples/inspect.py` prints, for each fixture certificate, either the decoded
subject, issuer, serial, version, validity, SAN, identity and chain verdict, or
the typed error that rejected the bytes.

## Dependencies

Python 3.9+ and pytest. The runtime uses the standard library only.

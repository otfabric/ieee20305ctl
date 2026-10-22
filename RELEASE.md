# ieee20305ctl Releases

Releases are cut with the `Release` workflow (semantic versioning, annotated
tag, GitHub release with binaries for every platform). The section of the
version below, up to the `---` that ends it, is the release's notes.

## v0.1.0

**Date:** unreleased  
**Previous release:** none

### Summary

First release. ieee20305ctl is a command-line tool for **IEEE 2030.5**
(Smart Energy Profile 2.0) built on [go-ieee20305](https://github.com/otfabric/go-ieee20305)
v0.2.0: a test PKI with IEEE 2030.5 device identities, offline validation of
IEEE 2030.5 and CSIP-AUS documents, and discovery of a server's resources over
mutual TLS.

### Highlights

- **`cert generate`:** A test PKI for mutual TLS: a CA, a server certificate
  for **`--server-name`** (host names and IP addresses, `localhost` and
  `127.0.0.1` by default) and **`--devices`** device certificates, all ECDSA
  P-256, as PEM files in **`--out`** with owner-only PKCS #8 keys. Existing
  files are kept unless **`--force`**. Prints the LFDI and SFDI of every
  certificate.
- **`cert inspect`** and **`cert lfdi`:** Subject, issuer, validity, public
  key, names and key usages of a PEM or DER certificate, and its LFDI and SFDI
  (the SFDI check digit as IEEE 2030.5 defines it).
- **`validate`:** Decodes IEEE 2030.5-2023 documents, or CSIP-AUS v1.3
  documents with **`--profile csipaus`**, and checks the constraints of the
  schema: cardinalities, string and binary lengths, numeric ranges and
  patterns. Exit status 5 when a document is invalid.
- **`client discover`:** Connects with the device certificate (**`--host`**,
  **`--cert`**, **`--key`**, **`--ca`**, **`--dcap`**, **`--timeout`**),
  retrieves the DeviceCapability resource and lists the function sets it
  links to, with hrefs and list sizes.
- **Output:** **`-o text|table|json`**; JSON results in the envelope of
  `docs/json-output.md`, and the exit codes of `docs/exitcodes.md` (0 to 5):
  a connection failure is 3, a certificate refused by either side 4.

### Limitations

- IEEE 2030.5 requires the cipher suite TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8,
  which Go does not implement. The client offers the other ECDHE-ECDSA
  suites; a server that accepts only CCM_8 refuses the connection.
- `validate` does not report a required element or attribute that is absent:
  it decodes to its zero value.
- `client register`, `programs`, `controls` and `report`, and `server`, are
  not implemented yet.

---

# ieee20305ctl

Command-line client, server and tooling for IEEE 2030.5 (Smart Energy Profile 2.0), built on [go-ieee20305](https://github.com/otfabric/go-ieee20305).

## Certificates

IEEE 2030.5 identifies devices by their certificate: the LFDI is the leftmost 160 bits of the SHA-256 fingerprint of the certificate, and the SFDI its leftmost 36 bits as a decimal number with a check digit.

```bash
ieee20305ctl cert generate --out pki --devices 2
```

`cert generate` writes a test PKI for mutual TLS: `ca.pem`/`ca.key`, `server.pem`/`server.key` (for `--server-name`, by default `localhost` and `127.0.0.1`) and `device-N.pem`/`device-N.key`, all ECDSA P-256, with owner-only key files. It does not overwrite existing files unless `--force` is given, and prints the LFDI and SFDI of every certificate.

```bash
ieee20305ctl cert inspect pki/server.pem
```

```bash
ieee20305ctl cert lfdi pki/device-1.pem -o json
```

Every command accepts `-o json` for a [JSON result](docs/json-output.md), and exits with the [documented exit codes](docs/exitcodes.md).

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

## Discovering a server

```bash
ieee20305ctl client discover --host utility.example:8443 --cert pki/device-1.pem --key pki/device-1.key --ca pki/ca.pem
```

`client discover` connects with mutual TLS, retrieves the server's DeviceCapability resource (`--dcap`, `/dcap` by default) and lists the function sets it links to, with their hrefs and, for lists, their number of items. A connection failure exits with status 3, a certificate either side refuses with status 4. IEEE 2030.5 requires the cipher suite TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8, which Go does not implement; the client offers the other ECDHE-ECDSA suites, so a server that accepts only CCM_8 refuses the connection.

## Registering a device

```bash
ieee20305ctl client register --host utility.example:8443 --cert pki/device-1.pem --key pki/device-1.key --ca pki/ca.pem --pin 111115
```

`client register` makes sure the server has an EndDevice for the device, identified by the SFDI and LFDI of its certificate: it looks it up in the server's EndDeviceList and creates it when absent (`--no-create` only looks it up). It then reads the device's Registration and, with `--pin`, checks that the server registered the PIN the device was given out of band. An unregistered device or a PIN mismatch exits with status 1.

## Validating documents

```bash
ieee20305ctl validate --profile csipaus derctl.xml edev.xml
```

`validate` decodes IEEE 2030.5-2023 documents, or CSIP-AUS v1.3 documents with `--profile csipaus`, and checks them against the constraints of the schema: cardinalities, string and binary lengths, numeric ranges and patterns. It exits with status 5 when a document is invalid. A required element or attribute that is absent decodes to its zero value and is not reported; use the official XSD for that.

## Output

Every command accepts `-o json` for a [JSON result](docs/json-output.md), and exits with the [documented exit codes](docs/exitcodes.md).

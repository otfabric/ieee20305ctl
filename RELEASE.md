# ieee20305ctl Releases

Releases are cut with the `Release` workflow (semantic versioning, annotated
tag, GitHub release with binaries for every platform). The section of the
version below, up to the `---` that ends it, is the release's notes.

## v0.3.1

**Date:** 2026-10-31
**Previous release:** v0.3.0

### Fixed

- A seed duration in seconds too large for `time.Duration` wrapped silently. It is now an error.
- Built on go-ieee20305 v0.5.3.

---

## v0.3.0

**Date:** 2026-10-30
**Previous release:** v0.2.1

### Summary

**Breaking for scripts:** the exit codes follow the OT Fabric organisation table (ADR-0006 of otfabric/engineering). Nothing else changes in the commands.

### Exit codes

| Case | Old | New | JSON error |
|---|---|---|---|
| `context_canceled` (interrupted before a result) | 10 | 130 | kind `interrupted`, was `internal` |

Every other exit code is unchanged. See [docs/exitcodes.md](docs/exitcodes.md).

### Environment

`IEEE20305CTL_FORMAT` is now the canonical name of the format variable; `IEEE20305CTL_OUTPUT_FORMAT` stays accepted, and `IEEE20305CTL_FORMAT` wins when both are set.

---

## v0.2.1

**Date:** 2026-10-23  
**Previous release:** v0.2.0

### Summary

On go-ieee20305 v0.5.2 and tlsccm v0.3.0: the binary no longer contains a
fork of `crypto/tls`, and takes the fixes of the library.

### Changed

- **tlsccm v0.3.0:** the CCM_8 cipher suite of `--ccm8` comes from a copy of
  Go 1.22.0's `crypto/tls` inside tlsccm, reviewed against every
  `crypto/tls` vulnerability Go has published since; the ALPN error quotes
  the client's protocols (GO-2025-4008).
- **go-ieee20305 v0.5.2:** `client run` takes the default control of the
  program of highest precedence also when another program has no controls;
  `server start` fails when its store fails at startup, and answers 500,
  not 400, for a response it cannot encode.

---

## v0.2.0

**Date:** 2026-10-23  
**Previous release:** v0.1.0

### Summary

ieee20305ctl does all of IEEE 2030.5 now, in both roles, on
[go-ieee20305](https://github.com/otfabric/go-ieee20305) v0.5.1: a device
that reads, registers, answers, reports and subscribes, a device agent, a
seeded server an operator acts on while it runs, and certificate tools. The
command line, the JSON output and the exit codes follow the other otfabric
tools (ocppctl, modbusctl, dnp3ctl), and a guide shows every scenario with
real output, against its own server and the GridAPPS-D and Envoy servers.

**Upgrade notes**

- `--output`/`-o` is now `--format` (`text`, `table`, `json`; or
  `IEEE20305CTL_FORMAT`).
- JSON results are the result itself, not `{"status", "data"}`; a failure is
  `{"error": {"kind", "code", "message"}, "ieee20305": {…}}`
  ([docs/json-output.md](docs/json-output.md)).
- Exit codes are those of the other tools: 2 usage, 3 invalid input, 4
  transport, 5 timeout, 6 refused or protocol, 10 internal
  ([docs/exitcodes.md](docs/exitcodes.md)). An invalid document of `validate`
  is 3 (was 5); a PIN mismatch is 6 (was 1).
- The client connects to `--url` (an `https://` URL, or `host[:port]`)
  instead of `--host` and `--dcap`, and `--ca` is required.
- `client discover` browses DNS-SD; what it did is now `client dcap`.
- `cert` JSON fields are snake_case (`not_before`, `is_ca`, …); the SFDI is
  a string.

### Added

- **Device:** `client dcap`, `time`, `register` (unchanged in purpose),
  `get`, `walk`, `programs`, `controls`, `respond`, `der show`, `der put`,
  `report`, `log-event`, `pricing`, `flow-reservation`, `put`, `post`,
  `delete`, `subscribe` (an HTTPS listener that streams notifications),
  `connection-point` (CSIP-AUS), and `discover` (DNS-SD).
- **Device agent:** `client run` registers, polls, schedules the events of
  every function set with primacy, supersession and randomization, posts
  the Responses they ask for, mirrors metering (`--report-power`) and
  subscribes (`--notify-listen`), and streams each step.
- **Server:** `server start` serves every resource of the WADL from a YAML
  or JSON seed: devices with PINs, editions and roles (device, aggregator,
  operator), DER programs with defaults and controls, tariffs; in-band
  registration, notifications, CSIP-AUS (`--profile csipaus`), CCM_8
  (`--ccm8`), DNS-SD (`--advertise`); a record per thing that happens.
  `server generate-config` writes a commented seed
  ([docs/server-config.md](docs/server-config.md)).
- **Operator:** `server devices`, `get`, `issue-control`, `cancel`,
  `set-default`, `provision`, `register`, through a control socket
  (`--control-socket`) that speaks JSON lines; changes need `--yes`.
- **Certificates:** `cert verify` checks a chain as an IEEE 2030.5 peer
  does; `cert generate --hmn` gives device certificates a
  HardwareModuleName, which `cert inspect` shows.
- **Everywhere:** `--format text|table|json`, JSON Lines for streams,
  environment variables for every connection flag (`IEEE20305CTL_URL`, …),
  `--ccm8`, `--profile csipaus`, `--edition 2018|2023`, `--debug`.
- **Docs:** a [guide](docs/guide.md) of common scenarios with real output,
  and a README in the form of the other tools.

### Quality

- The tests run the real command tree against a server they start, for
  every command and format and the error paths; coverage 85.3% with the
  race detector, enforced by `make check`.
- Checked by hand against the GridAPPS-D server (CCM_8, CSIP) and the Envoy
  server (CSIP-AUS) of ieee20305-interop v0.2.0. Running it against them
  found a bug of go-ieee20305 (a ResponseList with a DERControlResponse
  answered 500), fixed in v0.5.1.

---

## v0.1.0

**Date:** 2026-10-22  
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

# ieee20305ctl — IEEE 2030.5 Command-Line Tool: Act as a Device, Serve a Test Server, Operate It, and Check Certificates

[![Go Version](https://img.shields.io/badge/Go-1.26.8%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI Status](https://github.com/otfabric/ieee20305ctl/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/ieee20305ctl/actions/workflows/ci.yml)
[![Code Coverage](https://codecov.io/gh/otfabric/ieee20305ctl/graph/badge.svg)](https://codecov.io/gh/otfabric/ieee20305ctl)
[![Latest Release](https://img.shields.io/github/v/release/otfabric/ieee20305ctl?label=release)](https://github.com/otfabric/ieee20305ctl/releases)

ieee20305ctl speaks [IEEE 2030.5](https://standards.ieee.org/ieee/2030.5/5897/)
(Smart Energy Profile 2.0), the protocol between utilities, aggregators and
distributed energy resources (inverters, batteries, EV chargers, meters), in
editions 2018 and 2023, with the CSIP and CSIP-AUS profiles, in both roles.
It is built on [go-ieee20305](https://github.com/otfabric/go-ieee20305), and
is one static binary with no runtime dependencies.

**Docs:** [guide](docs/guide.md) (common scenarios, with real output) ·
[exit codes](docs/exitcodes.md) · [JSON output](docs/json-output.md) ·
[server configuration](docs/server-config.md) · [RELEASE.md](RELEASE.md)

## Why ieee20305ctl?

- **Talk to a utility server as a device** from a shell: read its
  DeviceCapability, register and check the PIN, list the DER programs,
  controls and tariffs it assigns, answer events, report metering and DER
  status, subscribe to notifications.
- **Run a device** (`client run`): an agent that polls, schedules events by
  primacy with randomization, and answers them, as a real inverter would, and
  tells you each step.
- **Serve a test server** (`server start`) from a YAML seed of devices,
  programs and tariffs, and **operate it while it runs**: issue and cancel
  controls, change defaults, provision devices.
- **Script it**: `--format json` on every command, JSON Lines for streams,
  and [stable exit codes](docs/exitcodes.md).
- **Interoperable**: CCM_8 for servers that offer nothing else (`--ccm8`),
  CSIP-AUS site limits and connection points (`--profile csipaus`), verified
  against the Envoy and GridAPPS-D servers.

## Quick start

```bash
# A test PKI and an example seed
ieee20305ctl cert generate --out pki --devices 1
ieee20305ctl server generate-config | sed 's#cert: device.crt#cert: pki/device-1.pem#' > seed.yaml

# Terminal 1: a server, with a control socket for the operator
ieee20305ctl server start --listen 127.0.0.1:8443 --cert pki/server.pem --key pki/server.key \
    --ca pki/ca.pem --config seed.yaml --control-socket /tmp/ieee20305.sock

# Terminal 2: the device
export IEEE20305CTL_URL=https://127.0.0.1:8443/dcap
export IEEE20305CTL_CERT=pki/device-1.pem IEEE20305CTL_KEY=pki/device-1.key IEEE20305CTL_CA=pki/ca.pem
ieee20305ctl client register --pin 111115      # the PIN must match
ieee20305ctl client controls                   # what the server asks of the device
ieee20305ctl client run --pin 111115           # a device that applies and answers them

# Terminal 3: the operator
ieee20305ctl server issue-control --control-socket /tmp/ieee20305.sock \
    --set opModMaxLimW=30 --duration 30m --yes
```

```console
$ ieee20305ctl client controls
/derp/1/derc/1  Active  "Limit to 50 %"
  2026-10-22T23:05:16Z for 1h0m0s (program /derp/1)
  opModMaxLimW=50.00 %
  responses 03 to /rsps/1/rsp
/derp/1/derc/2  Scheduled  "Disconnect"
  2026-10-23T00:05:16Z for 30m0s (program /derp/1)
  opModConnect=false
  responses 03 to /rsps/1/rsp
```

The [guide](docs/guide.md) walks through every scenario with its output.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [IEEE 2030.5 in two minutes](#ieee-20305-in-two-minutes)
- [Usage](#usage)
- [Commands](#commands)
- [Environment variables](#environment-variables)
- [Output and exit codes](#output-and-exit-codes)
- [Interoperability](#interoperability)
- [Limitations](#limitations)
- [Project structure](#project-structure)
- [Development](#development)
- [License](#license)

## Features

| Area | What ieee20305ctl does |
|------|------------------------|
| **Device (client)** | DeviceCapability and every function set it links (`dcap`), server time and clock offset (`time`), find or create the EndDevice and check the Registration PIN (`register`), read any resource as XML (`get`) or the whole tree (`walk`) |
| **DER** | Programs with primacy and default controls (`programs`), controls with their status and settings (`controls`), Responses (`respond`), DERCapability, DERSettings, DERStatus and DERAvailability (`der show`, `der put`) |
| **Other function sets** | Metering mirrors (`report`), LogEvents (`log-event`), tariffs with their intervals and prices (`pricing`), flow reservations (`flow-reservation`), any resource from a file (`put`, `post`, `delete`) |
| **Subscriptions** | An HTTPS notification listener that subscribes and streams notifications (`subscribe`) |
| **Device agent** | `client run`: registration, polling, event scheduling with primacy, supersession and randomization for DER, demand response, pricing, messaging, prepayment and flow reservation; Responses; metering; subscriptions |
| **Discovery** | DNS-SD browsing for `_smartenergy._tcp` (`client discover`); `server start --advertise` |
| **Server** | `server start`: a server of every resource of the IEEE 2030.5 WADL, seeded from YAML or JSON (devices with PINs and roles, programs, defaults, controls, tariffs), with in-band registration, subscriptions and notifications, event lifecycle and Responses, and an event stream |
| **Operator** | Through the control socket: `issue-control`, `cancel`, `set-default`, `provision`, `register`, `devices`, `get` |
| **Certificates** | A test PKI with optional HardwareModuleNames (`cert generate`), LFDI and SFDI (`cert lfdi`), the whole certificate (`cert inspect`), chain verification as an IEEE 2030.5 peer does it (`cert verify`) |
| **Documents** | Validation against IEEE 2030.5-2023 or CSIP-AUS v1.3 (`validate`) |
| **Profiles and transport** | IEEE 2030.5-2018 and 2023 (`--edition`), CSIP-AUS (`--profile csipaus`), CCM_8 (`--ccm8`), mutual TLS throughout |

## Installation

Download a pre-built binary from the [GitHub Releases page](https://github.com/otfabric/ieee20305ctl/releases). No Go toolchain is required.

| Platform | CPU | Download |
|---|---|---|
| Linux | x86-64 | `ieee20305ctl-<version>-linux-amd64.tar.gz` |
| Linux | 64-bit ARM | `ieee20305ctl-<version>-linux-arm64.tar.gz` |
| Linux | 32-bit ARM | `ieee20305ctl-<version>-linux-armv7.tar.gz` |
| macOS | Apple Silicon | `ieee20305ctl-<version>-darwin-arm64.tar.gz` |
| macOS | Intel | `ieee20305ctl-<version>-darwin-amd64.tar.gz` |
| Windows | x86-64 | `ieee20305ctl-<version>-windows-amd64.zip` |
| Windows | ARM64 | `ieee20305ctl-<version>-windows-arm64.zip` |

Each release has `ieee20305ctl-<version>-checksums.txt` (SHA-256).

### Linux

```bash
VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/otfabric/ieee20305ctl/releases/latest | sed 's|.*/v||')
case "$(uname -m)" in
  x86_64)         ARCH=amd64 ;;
  aarch64|arm64)  ARCH=arm64 ;;
  armv7l)         ARCH=armv7 ;;
esac
FILE="ieee20305ctl-${VERSION}-linux-${ARCH}.tar.gz"
BASE="https://github.com/otfabric/ieee20305ctl/releases/download/v${VERSION}"
curl -fsSLO "${BASE}/${FILE}" && curl -fsSLO "${BASE}/ieee20305ctl-${VERSION}-checksums.txt"
grep "${FILE}" "ieee20305ctl-${VERSION}-checksums.txt" | sha256sum -c -
mkdir -p ieee20305ctl-dl && tar -xzf "${FILE}" -C ieee20305ctl-dl
sudo install -m 0755 "$(find ieee20305ctl-dl -type f -name 'ieee20305ctl*' | head -n1)" /usr/local/bin/ieee20305ctl
```

### macOS

```bash
VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/otfabric/ieee20305ctl/releases/latest | sed 's|.*/v||')
case "$(uname -m)" in arm64) ARCH=arm64 ;; x86_64) ARCH=amd64 ;; esac
FILE="ieee20305ctl-${VERSION}-darwin-${ARCH}.tar.gz"
BASE="https://github.com/otfabric/ieee20305ctl/releases/download/v${VERSION}"
curl -fsSLO "${BASE}/${FILE}" && curl -fsSLO "${BASE}/ieee20305ctl-${VERSION}-checksums.txt"
grep "${FILE}" "ieee20305ctl-${VERSION}-checksums.txt" | shasum -a 256 -c -
mkdir -p ieee20305ctl-dl && tar -xzf "${FILE}" -C ieee20305ctl-dl
sudo install -m 0755 "$(find ieee20305ctl-dl -type f -name 'ieee20305ctl*' | head -n1)" /usr/local/bin/ieee20305ctl
```

### Windows (PowerShell)

```powershell
$version = (Invoke-RestMethod https://api.github.com/repos/otfabric/ieee20305ctl/releases/latest).tag_name.TrimStart('v')
$arch    = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$file = "ieee20305ctl-$version-windows-$arch.zip"
$base = "https://github.com/otfabric/ieee20305ctl/releases/download/v$version"
Invoke-WebRequest "$base/$file" -OutFile "$env:TEMP\$file"
Invoke-WebRequest "$base/ieee20305ctl-$version-checksums.txt" -OutFile "$env:TEMP\ieee20305ctl-checksums.txt"
$expected = ((Select-String -Path "$env:TEMP\ieee20305ctl-checksums.txt" -Pattern $file).Line -split '\s+')[0]
if ($expected -ne (Get-FileHash "$env:TEMP\$file" -Algorithm SHA256).Hash) { throw "Checksum mismatch for $file" }
$dir = "$env:LOCALAPPDATA\Programs\ieee20305ctl"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Expand-Archive "$env:TEMP\$file" -DestinationPath "$env:TEMP\ieee20305ctl-dl" -Force
$exe = Get-ChildItem "$env:TEMP\ieee20305ctl-dl" -Recurse -Filter 'ieee20305ctl*.exe' | Select-Object -First 1
Copy-Item $exe.FullName "$dir\ieee20305ctl.exe" -Force
```

### Build from source

Go 1.26.8 or later (the CCM_8 module needs it):

```bash
go install github.com/otfabric/ieee20305ctl@latest
```

or `git clone`, then `make build` (writes `bin/ieee20305ctl`).

Check that it works: `ieee20305ctl version`.

## IEEE 2030.5 in two minutes

- **HTTPS with mutual TLS, XML on the wire.** Everything starts at the
  server's **DeviceCapability** (`/dcap`); clients follow its links.
- **A device is its certificate.** The SHA-256 fingerprint gives its
  **LFDI** (40 hex digits) and **SFDI** (a decimal with a check digit).
  There are no passwords.
- **EndDevice and Registration.** The server holds an EndDevice per device;
  its Registration carries a **PIN** the device was given out of band and
  checks before it trusts the server.
- **Programs and controls.** FunctionSetAssignments give the device its
  programs. A DER program has a **primacy** (lower wins) and **controls**:
  events such as "limit output to 50 % from 14:00 for an hour". The device
  schedules them, and answers with **Responses**: received, started,
  completed, superseded. A **default control** applies when none does.
- **Polling or notifications.** Devices poll at the server's `pollRate`, or
  subscribe and receive **Notifications** on a listener of their own.
- **Metering.** Devices post a **MirrorUsagePoint** with their readings.
- **Profiles.** **CSIP** (California) constrains the standard; **CSIP-AUS**
  (Australia) adds site limits (dynamic operating envelopes) and the
  connection point (NMI). The cipher suite IEEE 2030.5 mandates is
  TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8.

## Usage

```text
ieee20305ctl <group> <command> [flags]
```

A client command connects to `--url` (an `https://` URL of the
DeviceCapability, or `host[:port]` for `https://host:port/dcap`) as the
device of `--cert` and `--key`, and verifies the server with `--ca`; it
never falls back to the system's roots. Every command has `--help` with
examples, and `--format text|table|json`. `--debug` shows the causes of an
error.

The scenarios, with their real output, are in the **[guide](docs/guide.md)**:
a test PKI, a development server, registering, reading programs and
controls, answering them, running an agent, operating the server, metering
and DER status, subscriptions, a CCM_8 server (GridAPPS-D), CSIP-AUS
(Envoy), scripting with JSON, and troubleshooting.

## Commands

| Command | Does |
|---------|------|
| `client discover` | Find servers on the local network with DNS-SD |
| `client dcap` | The server's DeviceCapability and its links |
| `client time` | The server's clock, and the local clock's offset from it |
| `client register` | Find or create the device's EndDevice; check the Registration's PIN (`--pin`) |
| `client get <href>` | Any resource, as XML |
| `client walk` | The resource tree, as a stream |
| `client programs` | The DER programs of the device, with their default controls |
| `client controls` | Their DER controls, with status and settings (`--active`, `--program`) |
| `client respond <control>` | Answer a control (`--status received|started|completed|…`) |
| `client der show`, `der put` | The DER resources of the device; put them from XML files |
| `client report` | Post metering: a MirrorUsagePoint (`--file`, or `--power W`) |
| `client log-event` | Post a LogEvent |
| `client pricing` | Tariffs, rate components, intervals and prices |
| `client flow-reservation` | Ask for a flow reservation |
| `client put`, `post`, `delete` | Write any resource from a file; delete one (`--yes`) |
| `client subscribe <resource>` | Subscribe, and stream the notifications |
| `client run` | Run a device agent, and stream what it does |
| `client connection-point` | The CSIP-AUS connection point of the device (`--set`) |
| `server start` | Serve IEEE 2030.5 from a seed; stream what happens |
| `server generate-config` | Write an example seed |
| `server devices`, `get` | Read the running server |
| `server issue-control`, `cancel`, `set-default` | Change what devices receive (`--yes`) |
| `server provision`, `register` | Add and register devices |
| `cert generate`, `inspect`, `lfdi`, `verify` | Certificates |
| `validate <file>…` | Check XML documents against the schema |
| `completion`, `version` | Shell completion; the version |

## Environment variables

Every flag of the connection can come from the environment, so that a shell
session sets the device once:

| Variable | Flag |
|----------|------|
| `IEEE20305CTL_URL` | `--url` |
| `IEEE20305CTL_CERT`, `IEEE20305CTL_KEY`, `IEEE20305CTL_CA` | `--cert`, `--key`, `--ca` |
| `IEEE20305CTL_CCM8` | `--ccm8` |
| `IEEE20305CTL_PROFILE`, `IEEE20305CTL_EDITION` | `--profile`, `--edition` |
| `IEEE20305CTL_TIMEOUT` | `--timeout` |
| `IEEE20305CTL_PIN` | `--pin` of `client register` and `client run` |
| `IEEE20305CTL_FORMAT` (`IEEE20305CTL_OUTPUT_FORMAT` stays accepted) | `--format` |
| `IEEE20305CTL_CONTROL_SOCKET` | `--control-socket` |
| `IEEE20305CTL_LISTEN`, `IEEE20305CTL_SERVER_CERT`, `IEEE20305CTL_SERVER_KEY`, `IEEE20305CTL_SERVER_CONFIG` | `--listen`, `--cert`, `--key`, `--config` of `server start` |

A flag given on the command line wins. A malformed value (a duration that
does not parse) exits 3.

## Output and exit codes

Results go to stdout: text by default, `--format table`, or `--format json`
([fields](docs/json-output.md)). Streams (`client run`, `subscribe`, `walk`,
`discover`, `server start`) write a record per event, JSON Lines with
`--format json`. Diagnostics go to stderr.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage: an unknown command or flag, a change without `--yes` |
| 3 | Invalid input: nothing was sent |
| 4 | Transport: no connection, a TLS handshake that failed |
| 5 | Timeout |
| 6 | The server refused (4xx, 5xx), broke the protocol, or has another PIN |
| 10 | Output or internal error |
| 130 | Interrupted before a result |

Details, error codes and the JSON error object: [docs/exitcodes.md](docs/exitcodes.md).

## Interoperability

ieee20305ctl is go-ieee20305, whose client and server are tested on every
push against [ANU Envoy](https://github.com/bsgip/envoy), the
[GridAPPS-D](https://github.com/GRIDAPPSD/ieee-2030_5-server-go) server and
client, [py20305](https://github.com/DERSecurity/py20305) and the
[EPRI client](https://github.com/epri-dev/IEEE-2030.5-Client), through the
images of [ieee20305-interop](https://github.com/otfabric/ieee20305-interop).
The [guide](docs/guide.md) shows ieee20305ctl itself against the GridAPPS-D
server (CCM_8, CSIP) and the Envoy server (CSIP-AUS).

## Limitations

- **The server keeps its state in memory**: it is a test and development
  server, not a head-end. Restarting it starts from the seed again.
- **Curves** (`opModVoltVar`, …) are read and shown, but neither the seed nor
  `issue-control` creates them.
- **DNS-SD** needs multicast on the local network; it finds nothing across
  routers or in most container networks.

## Project structure

```
ieee20305ctl/
├── main.go               build metadata, Execute
├── cmd/                  the command tree: root, validate
│   ├── client/           client commands
│   ├── server/           server start and the operator commands
│   └── cert/             certificate commands
├── internal/
│   ├── conn/             the connection of a device, and the classification of errors
│   ├── site/             the server: seed, events, control socket
│   ├── ieee/             DER control settings by schema name
│   ├── types/            the results, and how they render
│   ├── errs/             error kinds, codes and exit codes
│   ├── format/           text, table, JSON and streams
│   ├── runner/           running a command, and rendering a fatal error
│   └── config/           flags and environment variables from struct tags
└── docs/                 guide, exit codes, JSON output, server configuration
```

## Development

```bash
make check      # format, vet, lint, tests with the race detector, coverage (minimum 85%)
make build      # checks, then bin/ieee20305ctl
make build-all  # every release platform
```

The tests run the real command tree (`cmd.Run`) against a server it starts
itself, on a free port. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)

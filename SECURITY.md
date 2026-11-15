# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly by
emailing security@otfabric.io. Do not open a public issue.

We will acknowledge receipt within 48 hours and aim to provide a fix within
7 days for critical issues.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

## Security Considerations

### What the tool does

ieee20305ctl acts as an IEEE 2030.5 device (`client …`), serves a test
server (`server start`) that an operator drives through a control socket
(`server issue-control`, `cancel`, `set-default`, `provision`, `register`),
and makes and checks certificates (`cert …`). DER controls carry power
limits and operating modes that equipment acts on: whoever can issue them,
through the server or its control socket, can curtail or dispatch
generation and storage. Metering data and DER status show how a site uses
energy.

The protocol work is done by
[go-ieee20305](https://github.com/otfabric/go-ieee20305) (v0.8.0, with
tlsccm v0.4.0 for `--ccm8`); its
[SECURITY.md](https://github.com/otfabric/go-ieee20305/blob/main/SECURITY.md)
covers the TLS verification, device identity, authorization by LFDI and the
library's own limits. This file covers what the tool adds and which of its
defaults can be weakened.

### Threat model

| Peer | Trusted for | Held to |
|------|-------------|---------|
| The server (client commands) | What its certificate chain, verified against `--ca`, proves; with `--pin`, the PIN of the Registration | Mutual TLS, the schema (a document that breaks it is exit 6), the bounds below |
| The server's notifier (`client run --notify-listen`, `client subscribe`) | Notifications, when it presents a serverAuth certificate that `--ca` verifies | The listener's timeouts and sizes below; a notification of another subscription is not printed |
| Devices (`server start`) | Their own resources, by the LFDI of a certificate that `--ca` verifies (go-ieee20305) | The library's request limits, the server timeouts below |
| The operator, through the control socket | Everything the socket offers | Owner-only file permissions; nothing else |
| Hosts on the local link (`client discover`) | Nothing: DNS-SD answers are not authenticated | Their names are shown escaped; a server found this way is verified by TLS like any other |
| Files (seed, certificates, documents for `validate`) | What the operator gave the tool | Parsing; their strings are shown escaped in text output |

Mutual TLS proves who a peer is. It does not stop an authenticated peer from
sending large, slow or unusual traffic: the limits below are for that.

### Secure defaults, and how to weaken them

| Default | How it is weakened | What that allows |
|---------|-------------------|------------------|
| Client commands verify the server against `--ca` only, never the system roots; `--ca` is required | — (cannot be turned off) | |
| The notification listener requires a certificate that `--ca` verifies, with serverAuth: another device of the same CA cannot post to it | `client run --notify-allow-anonymous`, `client subscribe --listen-allow-anonymous` (some servers, such as GridAPPS-D, present none) | Anyone who reaches the listener can post notifications |
| The client does not check the Registration PIN unless told to | Give `--pin` to `client run` and `client register` to check it | Without `--pin`, any server whose certificate `--ca` verifies is taken as the device's server |
| `server start` serves only the devices of the seed (and `server provision`) | `server start --in-band` | Unknown devices of `--ca` post their EndDevice; they are not registered until `server register` |
| `server start` refuses notification URIs of loopback, link-local, multicast and unspecified addresses | `server start --notify-loopback` (for local tests) | The server sends notifications to its own host |
| `server start` does not advertise itself | `server start --advertise` | Hosts on the link learn the server's address over DNS-SD |
| AES-GCM cipher suites of crypto/tls | `--ccm8` (client and `server start`) | The CCM_8 suite of go-ieee20305/tlsccm, a copy of Go 1.22's crypto/tls with CCM_8 (go-ieee20305 SECURITY.md) |
| On Unix systems the control socket is created without group or other permissions, then set to mode 0600 | Choose its path with `--control-socket`; on Windows the file mode does not restrict access, so choose a directory only the operator can open | |
| Operator and client commands that change state on the server need `--yes` (`server issue-control`, `cancel`, `set-default`, `client delete`; docs/exitcodes.md) | — | |

Defaults that are open, and how to narrow them:

- `server start --listen` and `client subscribe --listen` bind `0.0.0.0:8443`
  by default: every interface. Give an address, such as `127.0.0.1:8443` for
  local tests.
- `--yes` is checked by the command, not by the server. The control socket
  has no confirmation and no users of its own: whoever can connect to it can
  issue and cancel controls and provision and register devices. Keep it in
  a directory only the operator can open.
- Connections are limited by default: 256 open at once to `server start`
  (`--max-connections`), 64 to the notification listener of `client
  subscribe` (`--listen-max-connections`) and `client run`
  (`--notify-max-connections`). More wait until one closes; 0 removes the
  limit.
- `cert generate` makes a test PKI, for development and testing only. Its
  keys are written readable by their owner only.

### Resource limits

| What | Limit | Where |
|------|-------|-------|
| The documents one client command reads for lists | 5,000 in all (exit 6, `invalid_response`, beyond) | `client` commands that follow lists |
| `client walk --depth` | 0 to 16 | |
| A response of the server | 1 MiB (go-ieee20305) | client commands |
| A notification | 1 MiB; a notification that breaks the schema is answered 400 | the notification listener |
| Notifications waiting to be written by `client subscribe` | 16; one that does not fit is counted and reported in a `notifications_dropped` record (docs/json-output.md) | |
| Notification listener connections | Headers within 10 s, the whole request within 10 s, the answer within 30 s, idle 1 minute, headers of 16 KiB at most | `client run --notify-listen`, `client subscribe` |
| `server start` connections | Headers within 10 s, the whole request within 30 s, the answer within 30 s, idle 2 minutes, headers of 64 KiB at most | |
| Open connections, TLS handshakes included | 256 (`server start --max-connections`), 64 (`client subscribe --listen-max-connections`, `client run --notify-max-connections`); more connections wait until one closes; 0: no limit | |
| A control socket request | One line of 1 MiB; 30 s of work on the server; `--timeout` (30 s) in the operator command | |
| DER control settings (`--set`, the seed) | Percentages 0–100 % (signed ones −100 % to 100 %), in hundredths at most; a power-of-ten multiplier of −9 to 9, so a power of at most 32,767 × 10^9 W; values outside are invalid input, exit 3 | `server issue-control`, `server set-default`, the seed |
| `client flow-reservation --power` | A value that fits a power-of-ten multiplier of −9 to 9 | |
| `server start` request bodies, lists, resources per device, notification attempts | The defaults of go-ieee20305 (its SECURITY.md and RELEASE.md, v0.6.0–v0.8.0) | |

Text and table output show strings that come from peers or files (hrefs,
descriptions, certificate names, DNS-SD names, event fields) as they are
when every character is printable, and quoted with escapes otherwise, so
that a control character is shown, not acted on by the terminal. JSON
output escapes them as JSON does.

### Known limitations

- **Documents are printed as they are.** `client get`, `client der show`
  and `server get` print a document as the peer sent it. XML
  admits carriage returns and the C1 control characters; use
  `--format json` to see them escaped. A fatal error's message on stderr
  is not escaped either; with `--format json` it is in the JSON envelope.
- **The server is a test server.** It keeps its state in memory and has no
  operator accounts; the control socket is its only administration.
- **DNS-SD is not authenticated.** `client discover` lists what hosts on the
  link announce.
- **Interoperability evidence is dated.** The guide's runs against Envoy and
  GridAPPS-D were recorded on 2026-10-22 with ieee20305ctl v0.2.0
  (README.md, Interoperability).

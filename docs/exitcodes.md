# Process exit codes

Every command of ieee20305ctl exits with one of these codes. They are a
**stable contract** for scripts: a change is called out in the release notes.

| Code | Kind (`error.kind`) | Meaning |
|------|---------------------|---------|
| 0 | | Success |
| 2 | `usage` | The command line is wrong: an unknown command or flag, a missing argument, a state-changing operator command without `--yes` |
| 3 | `invalid_input` | Nothing was sent: a flag, file, certificate, seed or document is invalid; `validate` found an invalid document; an operator command named a program or resource the server does not have |
| 4 | `transport` | No conversation with the peer: the connection was refused, the TLS handshake failed (a certificate refused by either end, or no common cipher suite), the listener or the control socket could not be opened |
| 5 | `timeout` | No answer in `--timeout` |
| 6 | `refused` | The server answered with an HTTP error (4xx or 5xx); the `ieee20305` object has the status |
| 6 | `protocol` | The server broke the protocol, or its answer cannot be trusted: a document that does not decode, a Registration whose PIN is not the device's, a device the server has not registered |
| 10 | `output`, `internal` | The result could not be written, or ieee20305ctl failed; also an interruption (Ctrl-C) before a one-shot command has its result |

`refused` and `protocol` both exit 6: the server was reached and talked to,
and the answer is not what the command needed.

## The error object

With `--format json` a failure is one JSON object on stdout, and nothing
else:

```json
{
  "error": {
    "kind": "refused",
    "code": "forbidden",
    "message": "POST /edev: the server answered 403 Forbidden"
  },
  "ieee20305": {
    "method": "POST",
    "path": "/edev",
    "status": 403
  }
}
```

With `--format text` or `table` it is one line on stderr: `error: <message>`.
`--debug` adds the causes of the error, one `cause:` line each.

| Field | Meaning |
|-------|---------|
| `error.kind` | One of the kinds above |
| `error.code` | A stable snake_case code; see below |
| `error.message` | For people; it may change between versions |
| `ieee20305` | Protocol detail, when there is some: for `refused`, the `method`, `path`, HTTP `status`, and the server's `reason_code` and `retry_after` when its Error document has them; for `pin_mismatch`, the Registration (the result `client register` would have written) |

## Codes

| Kind | Codes |
|------|-------|
| `usage` | `invalid_input` (what the command-line parser rejects), `confirmation_required` (`server issue-control`, `cancel`, `set-default`, `client delete` without `--yes`) |
| `invalid_input` | `invalid_input`, `invalid_format` (`--format`), `invalid_url` (`--url` not `https://` or `host[:port]`), `invalid_flag_combination` (`client report` with both or neither of `--file` and `--power`, `--listen-cert` without `--listen-key`, …), `invalid_config` (a seed that does not load or validate), `invalid_document` (an XML file that does not decode or validate), `invalid_certificate` (a certificate or key that does not load, `cert verify` of a chain that does not verify), `unknown_resource` (an operator command naming a program or href the server does not have, `client respond` to something that is not a DERControl), `input_file_open_failed`, `output_file_create_failed` |
| `transport` | `transport_connect_failed`, `transport_tls_failed`, `listen_failed`, `control_socket_failed` |
| `timeout` | `request_timeout` (no answer to a request that changes nothing), `outcome_unknown` (no answer to a request that may have changed the server: it was **not** sent again; check before you resend) |
| `refused` | `not_found` (404), `forbidden` (403), `bad_request` (400), `conflict` (409), `server_error` (5xx), `unavailable` (503, 429), `http_error` (any other status) |
| `protocol` | `invalid_response`, `pin_mismatch`, `not_registered` |
| `output` | `json_encode_failed`, `table_render_failed`, `output_write_failed`, `output_file_write_failed` |
| `internal` | `context_canceled` (interrupted before a result), `internal_unexpected` |

## Streaming commands

`client run`, `client subscribe`, `client walk`, `client discover` and
`server start` write one record per event. Interrupting them (Ctrl-C,
SIGTERM) or the end of `--duration` ends the stream normally: exit 0.

- `client run` keeps going through failures that waiting may fix (the server
  is down, answers 5xx, has not registered the device yet), and exits with
  the error's code on one it cannot fix: a PIN mismatch (6), a certificate
  the server refuses (4).
- `server start` exits 4 when it cannot listen or open its control socket,
  and 3 when its certificates or seed are invalid.

## `validate`

An invalid document is a result, not an error: `validate` writes the result
of every file (with `"valid": false` and the reason) and exits 3.

## `client register --pin`

A Registration whose PIN is not `--pin` exits 6 with the code
`pin_mismatch`: the server is not the one the device was given, and a device
must not act on what it serves. The JSON error carries the registration in
its `ieee20305` object.

## State-changing commands

`client report`, `respond`, `log-event`, `flow-reservation`, `put`, `post`,
`delete`, `der put` and `connection-point --set` send their request once.
When no answer arrives in `--timeout`, they exit 5 with `outcome_unknown`:
the server may have applied it. Read the resource back before you resend.

The operator commands that change what devices receive (`server
issue-control`, `cancel`, `set-default`) and `client delete` need `--yes`.

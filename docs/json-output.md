# `--format json` field conventions

These conventions are a **stable contract** for scripting. A breaking change
to a name or shape is called out in the release notes.

## General

| Concept | JSON fields |
|---------|-------------|
| Names | snake_case for everything ieee20305ctl adds (`end_device`, `duration_s`, `poll_rate_s`) |
| IEEE 2030.5 documents | `xml`: the document, indented, as the device's codec encodes it (`client get`, `client der show --xml`, `server get`) |
| Hrefs | `href`, `end_device`, `reply_to`, `location`: as the server wrote them, relative to the server |
| Identifiers | `lfdi`: 40 upper-case hex digits; `sfdi`: a decimal string; `mrid`: upper-case hex |
| Times | RFC 3339 in UTC (`2026-10-22T23:05:16Z`); the records of streaming commands have milliseconds |
| Durations | `duration_s`, `poll_rate_s`: integers, seconds |
| Controls | `settings`: an array of `{"name", "value", "text"}`, one per control of a DERControlBase that is set; see below |
| Lists | `links`, `programs`, `controls`, `tariffs`, `devices`, `documents`: **arrays**, empty rather than null |
| Fatal error | one object `{"error": {"kind", "code", "message"}, "ieee20305": {…}}` on stdout, nothing else; see [exitcodes.md](exitcodes.md) |

Fields that do not apply are omitted, not null.

## Settings

A DER control's settings are its opMod elements, by schema name:

```json
"settings": [
  {"name": "opModConnect", "value": true, "text": "true"},
  {"name": "opModMaxLimW", "value": 5000, "text": "50.00 %"},
  {"name": "opModExpLimW", "value": {"value": 1500, "multiplier": 0}, "text": "1500 W"},
  {"name": "opModVoltVar", "value": "/derp/1/dc/2", "text": "/derp/1/dc/2"}
]
```

| `value` | For |
|---------|-----|
| `true`, `false` | `opModConnect`, `opModEnergize` and the other booleans |
| a number | Percentages in **hundredths of a percent**, as the schema has them (`5000` is 50 %): `opModMaxLimW`, `opModFixedW`, `opModFixedV`, … |
| `{"value", "multiplier"}` | Powers, reactive powers and voltages: value × 10^multiplier (W, var, V): `opModTargetW`, and the CSIP-AUS site limits `opModExpLimW`, `opModImpLimW`, `opModGenLimW`, `opModLoadLimW` |
| a string | The href of a curve: `opModVoltVar`, `opModFreqWatt`, … |

`text` is the value for people. Scripts use `value`.

## Results

### `client dcap`

| Field | Meaning |
|-------|---------|
| `url` | The DeviceCapability read |
| `poll_rate_s` | Its pollRate |
| `links` | `{"name", "href", "all"}`: each link, with the number of items of a list (`all`) |

### `client time`

`server_time`, `local_time`, `offset_ms` (positive: the server is ahead),
`tz_offset_s`, `dst_offset_s`, `quality`.

### `client register`

| Field | Meaning |
|-------|---------|
| `end_device` | The href of the device's EndDevice |
| `created` | Whether the command posted it (in-band registration) |
| `lfdi`, `sfdi` | The device's identifiers |
| `registered` | Whether the server has registered it |
| `pin`, `registered_at` | Its Registration |
| `pin_checked`, `pin_matches` | Whether `--pin` was given, and matches |

### `client programs`

`programs`: `{"href", "mrid", "description", "primacy", "default_control"
(settings), "controls", "active"}`.

### `client controls`

`controls`: `{"program", "href", "mrid", "description", "start",
"duration_s", "status", "settings", "reply_to", "response_required"}`.
`status` is the EventStatus the server keeps: `Scheduled`, `Active`,
`Completed`, `Cancelled`, `CancelledRandom`, `Superseded`.

### `client pricing`

`tariffs`: `{"href", "description", "currency", "price_power_of_ten_multiplier",
"primacy", "rate_code", "rate_components"}`; a rate component
`{"href", "description", "intervals"}`; an interval `{"href", "description",
"start", "duration_s", "status", "tou_tier", "price"}`. `price` is the price
of the first ConsumptionTariffInterval, in 10^`price_power_of_ten_multiplier`
of the currency per kWh.

### `client der show`

`ders`: `{"href", "resources"}`; a resource `{"name"` (`capability`,
`settings`, `status`, `availability`), `"href", "set"` (whether the device has
put it), `"xml"` (with `--xml`)`}`.

### `client respond`, `report`, `log-event`, `flow-reservation`, `post`

`{"action", "target"` (the list posted to), `"location"` (what the server
created), `"detail"}`.

### `client put`, `delete`, `der put`

`{"action", "hrefs"}`.

### `client get`, `server get`

`{"href", "element"` (or `"resource"`, the WADL resource), `"xml"}`.

### `client connection-point`

`{"end_device", "href", "connection_point_id", "set"}`.

### `server devices`

`devices`: `{"end_device", "lfdi", "sfdi", "registered", "pin", "changed"}`.

### `server issue-control`, `cancel`, `set-default`, `provision`, `register`

`{"action", "href"}`: what the server created or changed.

### `cert generate`

`{"dir", "files"}`; a file `{"role"` (`ca`, `server`, `device`), `"name",
"cert", "key", "lfdi", "sfdi"}`.

### `cert inspect`

`subject`, `issuer`, `serial`, `not_before`, `not_after`, `expired`,
`public_key`, `is_ca`, `dns_names`, `ip_addresses`, `ext_key_usages`,
`hardware_module_name` (`{"type", "serial_number"}`), `lfdi`, `sfdi`.

### `cert lfdi`, `cert verify`

`{"lfdi", "sfdi"}`; `verify` adds `certificate` and `valid`.

### `validate`

`{"profile", "documents", "invalid"}`; a document `{"file", "element",
"valid", "error"}`.

## Streams

`client run`, `client subscribe`, `client walk`, `client discover` and
`server start` write **JSON Lines**: one compact object per line, as it
happens.

### Events (`client run`, `client subscribe`, `server start`)

Every record has `time` and `event`; the other fields are set as the event
has them: `href`, `resource`, `status`, `mrid`, `settings`, `start`, `end`,
`lfdi`, `address`, `code`, `message`.

| `event` | From | Fields |
|---------|------|--------|
| `listening` | run, subscribe, server | `address`; `message`: the URL |
| `started` | run | `href` (the EndDevice), `lfdi`, `message` |
| `assignments` | run | `message`: how many FunctionSetAssignments |
| `der_control`, `load_control`, `message`, `price`, `supply_override`, `flow_reservation` | run | An event of a function set: `status` (`Scheduled`, `Active`, `Completed`, `Cancelled`, `Superseded`), `mrid`, `resource` (the program), `start`, `end`; `settings` for a DER control |
| `default_control` | run | `href`, `settings` |
| `response`, `response_failed` | run | `code` (the Response status: 1 received, 2 started, 3 completed, 6 cancelled, 7 superseded), `status` (its name), `resource` (the function set); `message` when it failed |
| `poll`, `poll_failed` | run | `message`: the duration or the error (`poll` only with `--polls`) |
| `subscribed`, `unsubscribed` | subscribe | `href` (the Subscription), `resource` |
| `notification` | subscribe | `resource` (subscribed), `code` and `status` (0 `changed`, or why the server cancelled), `href` and `message` (the document carried) |
| `notification` | server | `resource` (WADL), `message`: the delivery's duration |
| `notification_failed`, `subscription_cancelled` | server | `resource`, `message` or `code` |
| `device_provisioned` | server | `href`, `lfdi`, `message` (SFDI, role, edition) |
| `control_issued`, `control_cancelled`, `default_set` | server | `href`, `mrid`, `resource`, `settings`, `start`, `end` |
| `response` | server | `href` (where stored), `mrid` (the event answered), `lfdi`, `code` |
| `event_status` | server | `resource` (the element), `status` (its new EventStatus) |
| `request` | server | `status` (the method), `resource`, `code` (the HTTP status), `message` (the duration): every request with `--log-requests`, and 5xx answers always |
| `log`, `warning`, `error` | server | `message`: what the server logs (registrations, subscriptions, failed notifications) |
| `control_socket`, `advertised` | server | `address` or `message` |

```json
{"time":"2026-10-22T23:10:05.048Z","event":"der_control","resource":"/derp/1","status":"Active","mrid":"122A93076031BBB49505D8861E1FB781","settings":[{"name":"opModExpLimW","value":{"value":1500,"multiplier":0},"text":"1500 W"}],"start":"2026-10-22T23:10:04Z","end":"2026-10-22T23:20:04Z"}
```

### `client walk`

`{"depth", "href", "element", "link", "items"}`: `items` for a list.

### `client discover`

`{"instance", "host", "port", "addresses", "url", "subtypes", "removed"}`.

# The server: seed file and control socket

`ieee20305ctl server start` serves IEEE 2030.5 from a **seed file**: the
devices it knows, and the DER programs and tariffs it assigns them. While it
runs, an operator acts on it through its **control socket**.

## Starting

```console
ieee20305ctl server start --cert server.pem --key server.key --ca ca.pem \
    --config seed.yaml --control-socket /tmp/ieee20305.sock
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--listen` | `0.0.0.0:8443` | Address to listen on (`127.0.0.1:0` picks a free port: see the `listening` record) |
| `--cert`, `--key` | | The server's certificate and key (PEM); required |
| `--ca` | | The CA certificates of the devices (PEM); required |
| `--config` | | The seed file; without it the server knows no device |
| `--profile` | `ieee20305` | `csipaus`: CSIP-AUS documents and connection points, IEEE 2030.5-2018 |
| `--edition` | `2023` | The edition of devices the seed does not give one |
| `--ccm8` | off | Serve TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first, then AES-128-GCM: what GridAPPS-D and EPRI clients need |
| `--in-band` | off | Let unknown devices post their EndDevice; they are registered with `server register` |
| `--assigned-devices-only` | off | An aggregator registers only the devices of the seed whose `managed_by` is its LFDI; without it, it may also register devices the seed does not name, which it then manages |
| `--control-socket` | | The Unix socket of the operator commands |
| `--notify-loopback` | off | Deliver notifications to loopback and link-local URIs, which the server refuses by default |
| `--advertise` | off | Advertise the server with DNS-SD (`_smartenergy._tcp`) |
| `--log-requests` | off | A `request` record per request |
| `--max-connections` | `256` | Most connections open at once, TLS handshakes included; more wait until one closes. 0: no limit |
| `--duration` | `0` | Stop after this long; 0: until interrupted |

Every flag can come from the environment: `IEEE20305CTL_LISTEN`,
`IEEE20305CTL_SERVER_CERT`, `IEEE20305CTL_SERVER_KEY`, `IEEE20305CTL_CA`,
`IEEE20305CTL_SERVER_CONFIG`, `IEEE20305CTL_PROFILE`, `IEEE20305CTL_EDITION`,
`IEEE20305CTL_CONTROL_SOCKET`, `IEEE20305CTL_FORMAT`.

A connection must send its request headers within 10 s and the whole
request within 30 s, and take its answer within 30 s; an idle connection is
closed after 2 minutes, and request headers are limited to 64 KiB (431
beyond). SECURITY.md lists these and the other limits.

The DeviceCapability is at `/dcap`. The server has the roots `/derp` (DER
programs), `/rsps` (where devices answer events), `/tp` (tariffs), `/mup`
(metering mirrors), `/edev` and `/tm`.

## The seed file

YAML or JSON (JSON is YAML). Durations are `90s`, `15m`, `1h`, or a number of
seconds. Unknown fields are an error. `server generate-config` writes this
example:

```yaml
# pollRate the server gives devices, in seconds.
poll_rate: 60

devices:
  - cert: device.crt        # or lfdi: 3E4F45AB31EDFE5B67E343E5E4562E31984E23E5
    pin: 111115

programs:
  - description: Utility program
    primacy: 1
    default:
      opModConnect: true
      opModMaxLimW: 100
    controls:
      - description: Limit to 50 %
        set:
          opModMaxLimW: 50
        start: 0s
        duration: 1h
      - description: Disconnect
        set:
          opModConnect: false
        start: 1h
        duration: 30m

tariffs:
  - description: Time of use
    currency: 36
    price_multiplier: -4
    primacy: 1
    rate_code: TOU1
    rate_components:
      - description: Import
        role_flags: "0001"
        flow_direction: 1
        intervals:
          - description: Off-peak
            start: 0s
            duration: 1h
            price: 1500
          - description: Peak
            start: 1h
            duration: 1h
            price: 4500
```

### `devices`

| Field | Meaning |
|-------|---------|
| `cert` or `lfdi` | The device, by its certificate (PEM) or its LFDI |
| `pin` | The PIN of its Registration, given to the device out of band |
| `edition` | `2023` (default) or `2018`; CSIP-AUS is always 2018 |
| `role` | `device` (default), `aggregator` (acts for the devices it manages) or `operator` (acts for every device; has no EndDevice) |
| `managed_by` | The LFDI of the aggregator that manages the device; an aggregator cannot register a device of the seed that names another (or none) |
| `unregistered` | Create the EndDevice but no Registration: the device waits until `server register` |
| `device_category` | The deviceCategory of its EndDevice, in hex (`00020000`) |

A device gets an EndDevice with its lists, a DER, and a
FunctionSetAssignments with every program and tariff of the seed.

### `programs`

| Field | Meaning |
|-------|---------|
| `description` | Required: operators name programs by it |
| `primacy` | Lower wins when controls of several programs conflict |
| `mrid` | The mRID, hex; derived from the description when absent |
| `default` | The DefaultDERControl: what applies when no control does |
| `controls` | The DERControls: `description`, `set`, `start` (from when the server starts), `duration` (required), `mrid`, `no_responses` |

A control asks for Responses (received, and started and completed) at
`/rsps/1/rsp` unless `no_responses: true`.

**Settings** (`default`, `set`) are the opMod elements of a DERControlBase,
by schema name:

| Kind | Settings | Value |
|------|----------|-------|
| On or off | `opModConnect`, `opModEnergize`, `opModGridConnectPermit`, `opModIslandPermit` | `true`, `false` |
| Percent | `opModMaxLimW`, `opModFixedW`, `opModFixedV`, `opModMaxLimPctWAbsorb`, `opModMaxLimPctVAInject`, `opModMaxLimPctVAAbsorb` | `50`, `12.5`, `-10` (`opModFixedW`): two decimals at most; 0 to 100, and -100 to 100 for `opModFixedW` and `opModFixedV` |
| Watts, vars, volts | `opModTargetW`, `opModTargetVar`, `opModTargetV`, `opModMaxLimWInject`, `opModMaxLimWAbsorb`, `opModMaxLimVarInject`, `opModMaxLimVarAbsorb` | `1500`, `-40000` (the multiplier is chosen, from -9 to 9: at most 32767 × 10^9) |
| Hundredths of a percent | `opModFixedVar` (-10000 to 10000), `opModMaxLimPctVarInject`, `opModMaxLimPctVarAbsorb` (0 to 10000) | `2500` (25 %) |
| CSIP-AUS site limits | `opModExpLimW`, `opModImpLimW`, `opModGenLimW`, `opModLoadLimW`, `opModStorageTargetW` | watts |

A value outside these ranges, which IEEE 2030.5 does not allow, is invalid
input (exit 3). `ieee20305ctl server issue-control --help` lists every
setting the tool knows. Curves (`opModVoltVar`, …) are not set from the seed.

### `tariffs`

| Field | Meaning |
|-------|---------|
| `description`, `mrid`, `primacy`, `rate_code` | The TariffProfile |
| `currency` | ISO 4217 number: 36 AUD, 840 USD, 978 EUR |
| `price_multiplier` | Prices are in 10^multiplier of the currency per kWh |
| `rate_components` | `description`, `role_flags` (hex), `flow_direction` (1 forward, 19 reverse), `intervals` |
| `intervals` | TimeTariffIntervals: `description`, `start`, `duration`, `tou_tier`, `price` (of its one ConsumptionTariffInterval) |

## The control socket

With `--control-socket PATH` the server listens on a Unix socket (mode
0600; on Unix systems it is created without group or other permissions) for
the operator commands. Whoever can connect to it can use every command:
`--yes` is checked by the operator command, not by the server.

| Command | What it does |
|---------|--------------|
| `server devices` | Lists the EndDevices with their LFDI, SFDI and Registration |
| `server get <href>` | Reads a resource as the server holds it (a device's Response, say) |
| `server issue-control --set name=value … --duration D [--start S] [--program P] --yes` | Creates a DERControl in a program (by href or description; the only one by default) |
| `server cancel <href> --yes` | Cancels a control |
| `server set-default --set name=value … [--program P] --yes` | Replaces the default control of a program |
| `server provision --cert C | --lfdi L [--pin N] [--role R] [--unregistered]` | Tells the server of a device |
| `server register --lfdi L --pin N` | Registers a device (in-band, or provisioned `unregistered`) |

Each takes `--control-socket` (or `IEEE20305CTL_CONTROL_SOCKET`) and
`--format`. A control socket that cannot be opened, or that closes without a
reply, is exit 4 (`control_socket_failed`). A request the server took and
did not answer within `--timeout` is exit 5: `outcome_unknown` for
`issue-control`, `cancel`, `set-default`, `provision` and `register` (the
server may have acted: read its state before you repeat the command),
`request_timeout` for `devices` and `get`. Ctrl-C while the command waits
is exit 130.

A Unix socket path is limited to about 100 bytes (104 on macOS, 108 on
Linux): use a short path such as `/tmp/ieee20305.sock`.

The protocol is one JSON object per line each way, so other tools can use the
socket too:

```console
$ echo '{"op":"devices"}' | nc -U /tmp/ieee20305.sock
{"ok":true,"result":[{"end_device":"/edev/1","lfdi":"518C61748B9C454A99D44A5260F0E8996E5F89B7","sfdi":"218904717524","registered":true,"pin":111115,"changed":"2026-10-22T23:21:00Z"}]}
```

| `op` | Fields | `result` |
|------|--------|----------|
| `devices` | | the devices |
| `get` | `href` | `{"href", "resource", "xml"}` |
| `issue-control` | `control`: `{"program", "description", "set": ["name=value"], "start_at" (RFC 3339), "duration" (nanoseconds), "no_responses"}` | `{"href"}` |
| `cancel` | `href` | `{"href"}` |
| `set-default` | `program`, `set` | `{"href"}` |
| `provision` | `device`: a device of the seed | `{"href"}` |
| `register` | `lfdi`, `pin` | `{"href"}` |

A failure is `{"ok": false, "error": {"kind", "code", "message"}}`.

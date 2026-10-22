# ieee20305ctl guide: common scenarios

Each scenario shows the commands and what they print. The output is real:
recorded against ieee20305ctl's own server and, where noted, against the
GridAPPS-D and Envoy servers of
[ieee20305-interop](https://github.com/otfabric/ieee20305-interop) v0.2.0.
Identifiers, times and ports differ on every run.

1. [A test PKI](#1-a-test-pki)
2. [A server for development](#2-a-server-for-development)
3. [Connect as a device and register](#3-connect-as-a-device-and-register)
4. [What the server tells the device: programs, controls, prices](#4-what-the-server-tells-the-device-programs-controls-prices)
5. [Answer a control](#5-answer-a-control)
6. [Run a device agent](#6-run-a-device-agent)
7. [Operate the server: issue, cancel, set defaults](#7-operate-the-server-issue-cancel-set-defaults)
8. [Report metering, log events, DER status](#8-report-metering-log-events-der-status)
9. [Subscriptions and notifications](#9-subscriptions-and-notifications)
10. [A server that offers only CCM_8 (GridAPPS-D)](#10-a-server-that-offers-only-ccm_8-gridapps-d)
11. [CSIP-AUS: site limits and connection points (Envoy)](#11-csip-aus-site-limits-and-connection-points-envoy)
12. [Scripting: JSON and exit codes](#12-scripting-json-and-exit-codes)
13. [Troubleshooting](#13-troubleshooting)

---

## 1. A test PKI

IEEE 2030.5 is mutual TLS: the server and every device have a certificate. A
device is known by the LFDI and SFDI of its certificate's fingerprint.

```console
$ ieee20305ctl cert generate --out pki --devices 2
Wrote to pki:

ROLE    CERTIFICATE   KEY           LFDI                                      SFDI
ca      ca.pem        ca.key        1007D2EA618C97F53D45E40E56E91700AB44A995  43031712388
server  server.pem    server.key    E530AA8E10410B599277CA3281F3F4FE212D0245  615227496657
device  device-1.pem  device-1.key  518C61748B9C454A99D44A5260F0E8996E5F89B7  218904717524
device  device-2.pem  device-2.key  2C2E719BCEC2D37A1D297B0A03FF2DE7FDCFB6C9  118598599005
```

The server certificate is for `localhost` and `127.0.0.1` unless
`--server-name` says otherwise. `--hmn` gives the device certificates a
HardwareModuleName, as the certificates of real IEEE 2030.5 devices have.

The identifiers of any certificate, and whether a chain verifies the way a
peer verifies it:

```console
$ ieee20305ctl cert lfdi pki/device-1.pem
LFDI: 518C61748B9C454A99D44A5260F0E8996E5F89B7
SFDI: 218904717524

$ ieee20305ctl cert verify pki/server.pem --ca pki/ca.pem --usage server --name localhost
pki/server.pem: verified
  LFDI E530AA8E10410B599277CA3281F3F4FE212D0245
  SFDI 615227496657
```

`cert inspect` shows the whole certificate.

## 2. A server for development

Write the example seed, point it at the device certificate, and serve it:

```console
$ ieee20305ctl server generate-config --output seed.yaml
$ sed -i.bak 's#cert: device.crt#cert: pki/device-1.pem#' seed.yaml
$ ieee20305ctl server start --listen 127.0.0.1:8443 --cert pki/server.pem --key pki/server.key \
      --ca pki/ca.pem --config seed.yaml --control-socket /tmp/ieee20305.sock
23:05:06.710 listening      127.0.0.1:8443 https://127.0.0.1:8443/dcap (ieee20305, 2023)
23:05:06.711 default_set    /derp/1/dderc opModConnect=true opModMaxLimW=100.00 %
23:05:06.711 control_issued /derp/1/derc/1 resource /derp/1 mrid 3BDDD411EA231F498B6378B9B888EE69 opModMaxLimW=50.00 % 23:05:06..00:05:06
23:05:06.711 event_status   Active resource DERControl
23:05:06.711 control_issued /derp/1/derc/2 resource /derp/1 mrid 3133625DC984284035B301A5CD0BCBFD opModConnect=false 00:05:06..00:35:06
23:05:06.711 event_status   Active resource TimeTariffInterval
23:05:06.712 log            device 518C61748B9C454A99D44A5260F0E8996E5F89B7 provisioned at /edev/1 (device, 2023)
23:05:06.712 device_provisioned /edev/1 lfdi 518C61748B9C454A99D44A5260F0E8996E5F89B7 SFDI 218904717524, device, 2023
```

The server keeps running and writes a record for everything that happens.
The seed file is described in [server-config.md](server-config.md).

For the next scenarios, give the client commands the device's connection
in the environment, so that the commands stay short:

```console
$ export IEEE20305CTL_URL=https://127.0.0.1:8443/dcap
$ export IEEE20305CTL_CERT=pki/device-1.pem IEEE20305CTL_KEY=pki/device-1.key IEEE20305CTL_CA=pki/ca.pem
$ export IEEE20305CTL_CONTROL_SOCKET=/tmp/ieee20305.sock
```

## 3. Connect as a device and register

What does the server offer?

```console
$ ieee20305ctl client dcap
DeviceCapability https://127.0.0.1:8443/dcap (poll every 60 s)
  DERProgramListLink                 /derp (1)
  ResponseSetListLink                /rsps (1)
  TariffProfileListLink              /tp (1)
  TimeLink                           /tm
  EndDeviceListLink                  /edev (1)
  MirrorUsagePointListLink           /mup (0)
```

Register, and check that the server is the one the device was given: its
Registration must carry the PIN the device got out of band.

```console
$ ieee20305ctl client register --pin 111115
EndDevice /edev/1 (found)
  LFDI 518C61748B9C454A99D44A5260F0E8996E5F89B7
  SFDI 218904717524
  registered, PIN 111115 matches

$ ieee20305ctl client register --pin 999999
error: the Registration of /edev/1 has PIN 111115, not 999999: the server is not the one the device was given
$ echo $?
6
```

Is the device's clock in step with the server's? Devices schedule events by
the server's time.

```console
$ ieee20305ctl client time
server time  2026-10-22T23:05:24Z
local time   2026-10-22T23:05:24Z
offset       0s (the server is in step)
time zone    +0 s, DST +0 s
quality      7
```

## 4. What the server tells the device: programs, controls, prices

```console
$ ieee20305ctl client programs
/derp/1  primacy 1  "Utility program"
  default   opModConnect=true opModMaxLimW=100.00 %
  controls  2 (1 active)

$ ieee20305ctl client controls
/derp/1/derc/1  Active  "Limit to 50 %"
  2026-10-22T23:05:16Z for 1h0m0s (program /derp/1)
  opModMaxLimW=50.00 %
  responses 03 to /rsps/1/rsp
/derp/1/derc/2  Scheduled  "Disconnect"
  2026-10-23T00:05:16Z for 30m0s (program /derp/1)
  opModConnect=false
  responses 03 to /rsps/1/rsp

$ ieee20305ctl client pricing
/tp/1  "Time of use"  primacy 1  currency 36
  /tp/1/rc/1  "Import"
    Active  2026-10-22T23:05:16Z for 1h0m0s  tier 1  0.15/kWh  "Off-peak"
    Scheduled  2026-10-23T00:05:16Z for 1h0m0s  tier 1  0.45/kWh  "Peak"
```

`client controls --active` lists only what applies now; `--program` selects a
program by href or description. Any resource can be read as XML:

```console
$ ieee20305ctl client get /edev/1/rg
<Registration xmlns="urn:ieee:std:2030.5:ns" schemaVer="2.2" href="/edev/1/rg">
  <dateTimeRegistered>1792710316</dateTimeRegistered>
  <pIN>111115</pIN>
</Registration>
```

`client walk` visits everything the server offers the device:

```console
$ ieee20305ctl client walk --depth 2 --format table
DEPTH     HREF                          ELEMENT           LINK      ITEMS
-----     ----                          -------           ----      -----
0         https://127.0.0.1:8443/dcap   DeviceCapability
1         /derp                         DERProgramList    DERProgramListLink  1
1         /rsps                         ResponseSetList   ResponseSetListLink  1
1         /tp                           TariffProfileList  TariffProfileListLink  1
1         /tm                           Time              TimeLink
1         /edev                         EndDeviceList     EndDeviceListLink  1
…
```

## 5. Answer a control

A control with a `replyTo` asks the device for Responses: received, started,
completed (or opted out, cancelled, superseded).

```console
$ ieee20305ctl client respond /derp/1/derc/1 --status started
response: posted to /rsps/1/rsp, created /rsps/1/rsp/1
  status 2 (started) to control E0F6AAB1736815FF605385547F1ADF2A
```

The server's stream shows it:

```text
23:10:05.049 response       /rsps/1/rsp/1 mrid E0F6AAB1736815FF605385547F1ADF2A lfdi 518C61748B9C454A99D44A5260F0E8996E5F89B7 2
```

## 6. Run a device agent

`client run` is a device: it registers, polls at the server's rate (or
`--poll-rate`), schedules every event with primacy and randomization, posts
the Responses they ask for, and with `--report-power` mirrors its metering.
Here it runs for eight seconds while the operator issues a 30 % limit
after four (scenario 7):

```console
$ ieee20305ctl client run --pin 111115 --poll-rate 2s --report-power 4200 --duration 8s --polls
23:24:13.334 started        /edev/1 lfdi 518C61748B9C454A99D44A5260F0E8996E5F89B7 registered with PIN 111115
23:24:13.334 assignments    1 FunctionSetAssignments
23:24:13.335 der_control    Scheduled resource /derp/1 mrid 7A131FCAD343422FF1771A5473ADF761 opModMaxLimW=50.00 % 23:24:11..00:24:11
23:24:13.336 response       received resource DER 1
23:24:13.336 der_control    Active resource /derp/1 mrid 7A131FCAD343422FF1771A5473ADF761 opModMaxLimW=50.00 % 23:24:11..00:24:11
23:24:13.337 response       started resource DER 2
23:24:13.337 der_control    Scheduled resource /derp/1 mrid 637EB04D831635E2048E78EA55E87A47 opModConnect=false 00:24:11..00:54:11
23:24:13.337 response       received resource DER 1
23:24:13.338 default_control /derp/1/dderc opModConnect=true opModMaxLimW=100.00 %
23:24:15.341 assignments    1 FunctionSetAssignments
23:24:15.346 poll           7ms
23:24:17.348 assignments    1 FunctionSetAssignments
23:24:17.350 der_control    Scheduled resource /derp/1 mrid F1150A42246F91B46A128DD3213D6C54 opModMaxLimW=30.00 % 23:24:17..23:34:17
23:24:17.350 response       received resource DER 1
23:24:17.350 der_control    Active resource /derp/1 mrid F1150A42246F91B46A128DD3213D6C54 opModMaxLimW=30.00 % 23:24:17..23:34:17
23:24:17.351 response       started resource DER 2
23:24:17.351 der_control    Superseded resource /derp/1 mrid 7A131FCAD343422FF1771A5473ADF761 opModMaxLimW=50.00 % 23:24:11..00:24:11
23:24:17.351 response       superseded resource DER 7
23:24:17.352 poll           4ms
23:24:19.354 assignments    1 FunctionSetAssignments
23:24:19.359 poll           6ms
```

The 30 % limit issued at 23:24:17 conflicts with the 50 % limit of the same
program (both set `opModMaxLimW`): the newer one wins, and the agent tells
the server so with Response 7 (superseded). A control of a program with a
lower primacy number would win over both.

The price and flow-reservation events of the device are in the stream too
(`price`, `flow_reservation`). Without `--duration` it runs until
interrupted; with `--format json` each record is a JSON line, ready for
`jq`. It stops with exit 6 when the server's PIN is not `--pin`.

With `--notify-listen` the agent serves a notification listener, subscribes
to its programs, and polls less (`--subscribed-poll-rate`).

## 7. Operate the server: issue, cancel, set defaults

The operator commands talk to the running `server start` through its
control socket. What changes what devices receive needs `--yes`:

```console
$ ieee20305ctl server devices
/edev/1  LFDI 518C61748B9C454A99D44A5260F0E8996E5F89B7  SFDI 218904717524  registered, PIN 111115

$ ieee20305ctl server issue-control --set opModMaxLimW=30 --duration 30m
error: issue-control changes what devices receive: confirm with --yes

$ ieee20305ctl server issue-control --set opModExpLimW=1500 --duration 30m --yes
error: ieee20305: invalid: opModExpLimW are CSIP-AUS settings: start the server with --profile csipaus

$ ieee20305ctl server issue-control --set opModMaxLimW=30 --duration 30m --description Curtail --yes
issued: /derp/1/derc/3

$ ieee20305ctl server cancel /derp/1/derc/3 --yes
cancelled: /derp/1/derc/3

$ ieee20305ctl server set-default --set opModConnect=true --set opModMaxLimW=80 --yes
default set: /derp/1/dderc

$ ieee20305ctl server provision --cert pki/device-2.pem --pin 222226
provisioned: /edev/2
```

`--start 5m` issues a control that starts in five minutes; `--program`
names the program when the server has several. The settings are listed by
`ieee20305ctl server issue-control --help` and in
[server-config.md](server-config.md#programs).

What did the device answer? The server holds each Response:

```console
$ ieee20305ctl server get /rsps/1/rsp/1
<DERControlResponse xmlns="urn:ieee:std:2030.5:ns" schemaVer="2.2" href="/rsps/1/rsp/1"><createdDateTime>1792710331</createdDateTime><endDeviceLFDI>518C61748B9C454A99D44A5260F0E8996E5F89B7</endDeviceLFDI><status>2</status><subject>E0F6AAB1736815FF605385547F1ADF2A</subject></DERControlResponse>
```

## 8. Report metering, log events, DER status

```console
$ ieee20305ctl client report --power 4200
report: posted to /mup, created /mup/1
  real power 4200 W

$ ieee20305ctl client log-event --code 1 --id 7 --details over-temperature
log-event: posted to /edev/1/lel, created /edev/1/lel/1
  code 1, id 7

$ ieee20305ctl client flow-reservation --energy 20000 --power 7000 --start 1h --duration 4h
flow-reservation: posted to /edev/1/frq, created /edev/1/frq/1
  20000 Wh at 7000 W from 2026-10-23T00:05:31Z for 4h0m0s
```

`client report --file mup.xml` posts a MirrorUsagePoint of your own.

The DER resources of the device, before and after it puts its settings:

```console
$ ieee20305ctl client der show
DER /edev/1/der/1
  capability       /edev/1/der/1/dercap         not set
  settings         /edev/1/der/1/derg           not set
  status           /edev/1/der/1/ders           not set
  availability     /edev/1/der/1/dera           not set

$ ieee20305ctl client der put --settings derg.xml --capability dercap.xml
put: /edev/1/der/1/dercap, /edev/1/der/1/derg
```

Every document is checked against the schema before it is sent;
`ieee20305ctl validate derg.xml` checks it alone. `client put` and
`client post` write any other resource from a file.

## 9. Subscriptions and notifications

`client subscribe` serves an HTTPS listener, subscribes to a resource, and
writes each notification. The server must reach the listener: give
`--notify-url` when `--listen` is not that address. (The development server
here needs `--notify-loopback` to post to `127.0.0.1`.)

```console
$ ieee20305ctl client subscribe /derp/1/derc --listen 127.0.0.1:18444 --count 1
23:10:48.135 listening      https://127.0.0.1:18444/notify notifications to https://127.0.0.1:18444/notify
23:10:48.135 subscribed     /edev/1/sub/2 resource /derp/1/derc
23:10:49.876 notification   changed /derp/1/derc resource /derp/1/derc DERControlList
23:10:49.876 unsubscribed   /edev/1/sub/2
```

The notification arrived when the operator issued a control
(`server issue-control … --yes`). The subscription is deleted when the
command ends, unless `--keep`.

## 10. A server that offers only CCM_8 (GridAPPS-D)

IEEE 2030.5 mandates TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8, which Go's TLS
stack lacks; some servers offer nothing else. Recorded against the
GridAPPS-D server of ieee20305-interop, seeded with its `csip` fixture:

```console
$ ieee20305ctl client dcap
error: the TLS handshake failed: … remote error: tls: handshake failure (the server may refuse the certificate, or offer only CCM_8: try --ccm8)
$ echo $?
4

$ ieee20305ctl client dcap --ccm8
DeviceCapability https://127.0.0.1:19443/dcap (poll every 900 s)
  MessagingProgramListLink           /msg
  ResponseSetListLink                /rsps
  TimeLink                           /tm
  UsagePointListLink                 /upt
  EndDeviceListLink                  /edev
  MirrorUsagePointListLink           /mup
  SelfDeviceLink                     /sdev

$ ieee20305ctl client programs --ccm8
/edev/59471700/fsa/0C08DE8C76C2481D68F4A8D4BB2E951D/derp/derp-22e309bfbb900388  primacy 1  "CSIP utility program"
  default   opModConnect=true opModMaxLimW=100.00 %
  controls  2 (1 active)
/edev/59471700/fsa/0C08DE8C76C2481D68F4A8D4BB2E951D/derp/derp-ba243beea7d02afe  primacy 2  "CSIP aggregator program"
  default   opModEnergize=true
  controls  1 (1 active)

$ ieee20305ctl client run --ccm8 --pin 111115 --duration 5s
23:18:45.373 started        /edev/59471700 lfdi DD8CA21CA64C5B63055E1462CBA559F4D1BAE7D5 registered with PIN 111115
23:18:45.375 der_control    Scheduled resource /edev/59471700/…/derp/derp-22e309bfbb900388 mrid 5E1275F1CE2DCC8A3ED20D4300009F1C opModMaxLimW=50.00 % 23:18:39..00:18:39
23:18:45.376 response       received resource DER 1
23:18:45.376 der_control    Active resource /edev/59471700/…/derp/derp-22e309bfbb900388 mrid 5E1275F1CE2DCC8A3ED20D4300009F1C opModMaxLimW=50.00 % 23:18:39..00:18:39
23:18:45.378 response       started resource DER 2
23:18:45.380 der_control    Active resource /edev/59471700/…/derp/derp-ba243beea7d02afe mrid 0F99D7AD9982FA91F4FE458B00009F1C opModFixedW=25.00 % 23:18:39..23:48:39
23:18:45.380 response       started resource DER 2
```

Two programs, primacy 1 and 2, each with an active control; the agent
starts both, since a fixed power and a limit do not conflict. `server start
--ccm8` serves CCM_8 too, then AES-128-GCM for clients without it.

## 11. CSIP-AUS: site limits and connection points (Envoy)

With `--profile csipaus` the tool reads and writes the documents of
CSIP-AUS: the site limits (dynamic operating envelopes) of controls, and
the connection point of the EndDevice. Recorded against the Envoy server of
ieee20305-interop, seeded with its `csip-aus` fixture:

```console
$ export IEEE20305CTL_PROFILE=csipaus IEEE20305CTL_EDITION=2018
$ ieee20305ctl client programs
/edev/1/derp/1  primacy 1  "DNSP envelope"
  default   opModExpLimW=1500 W opModGenLimW=5000 W opModImpLimW=10000 W opModLoadLimW=10000 W
  controls  2 (1 active)
/edev/1/derp/2  primacy 2  "Aggregator envelope"
  default   opModLoadLimW=8000 W
  controls  1 (1 active)

$ ieee20305ctl client controls --active
/edev/1/derp/1/derc/1  Active  "2026-10-22T23:19:14+00:00"
  2026-10-22T23:19:14Z for 1h0m0s (program /edev/1/derp/1)
  opModExpLimW=5000 W opModImpLimW=10000 W
  responses 03 to /edev/1/rsps/site_control/rsp
/edev/1/derp/2/derc/3  Active  "2026-10-22T23:19:14+00:00"
  2026-10-22T23:19:14Z for 30m0s (program /edev/1/derp/2)
  opModConnect=true opModLoadLimW=7000 W
  responses 03 to /edev/1/rsps/site_control/rsp

$ ieee20305ctl client connection-point --set 4407123456
connection point /edev/1/cp: 4407123456

$ ieee20305ctl client pricing
/edev/1/tp/1  "Time of use"  primacy 1  currency 36
  /edev/1/tp/1/rc/2  "Export"
    Active  2026-10-22T23:19:14Z for 2h0m0s  tier 0  0.05/kWh
  /edev/1/tp/1/rc/1  "Import"
    Active  2026-10-22T23:19:14Z for 1h0m0s  tier 0  0.25/kWh
    Scheduled  2026-10-23T00:19:14Z for 1h0m0s  tier 0  0.45/kWh
```

`server start --profile csipaus` serves CSIP-AUS; its seed sets limits like
any other setting (`opModExpLimW: 1500`).

## 12. Scripting: JSON and exit codes

Every command takes `--format json` (or `IEEE20305CTL_FORMAT=json`); the
fields are a stable contract ([json-output.md](json-output.md)). Errors are
JSON too, and the exit code says what happened ([exitcodes.md](exitcodes.md)):

```console
$ ieee20305ctl client controls --active --format json | jq -r '.controls[] | "\(.href) \(.settings[0].text)"'
/derp/1/derc/1 50.00 %

$ IEEE20305CTL_CERT=pki/device-2.pem IEEE20305CTL_KEY=pki/device-2.key ieee20305ctl client register --format json
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
$ echo $?
6
```

A health check of a device's view of its server:

```bash
#!/usr/bin/env bash
set -u
out=$(ieee20305ctl client register --pin "$PIN" --format json)
case $? in
  0) echo "registered at $(jq -r .end_device <<<"$out")" ;;
  4) echo "cannot reach the server or TLS failed: $(jq -r .error.message <<<"$out")"; exit 2 ;;
  6) echo "the server refused or is not ours: $(jq -r .error.code <<<"$out")"; exit 2 ;;
  *) echo "check the command: $(jq -r .error.message <<<"$out")"; exit 3 ;;
esac
```

The streams (`client run`, `subscribe`, `server start`) write JSON Lines:

```console
$ ieee20305ctl client run --pin 111115 --format json | jq -c 'select(.event=="der_control" and .status=="Active") | {mrid, settings}'
{"mrid":"E0F6AAB1736815FF605385547F1ADF2A","settings":[{"name":"opModMaxLimW","value":5000,"text":"50.00 %"}]}
```

## 13. Troubleshooting

| Symptom | Exit | Cause and fix |
|---------|------|---------------|
| `error: --url is required` | 3 | Give `--url` or `IEEE20305CTL_URL`, and `--cert`, `--key`, `--ca` |
| `connect: connection refused` | 4 | Nothing listens at `--url`: check the host and port |
| `the TLS handshake failed … try --ccm8` | 4 | The server offers only CCM_8 (GridAPPS-D, EPRI): add `--ccm8`. Or the server does not accept the device's certificate: is its CA the server's? |
| `x509: certificate signed by unknown authority` | 4 | `--ca` does not hold the CA of the server's certificate |
| `POST /edev: the server answered 403 Forbidden` | 6 | The server was not told of this device: provision it (`server provision`), or start the server with `--in-band` |
| `has PIN 111115, not 999999` | 6 | The server is not the one the device was given, or `--pin` is wrong |
| `the server has no EndDevice for SFDI …` | 6 | Run `client register` first |
| `the server answered 404 Not Found` | 6 | The href is not on the server, or it belongs to another device |
| `no answer in time; the server may have applied the request` | 5 | `outcome_unknown`: read the resource back before you resend |
| `listen unix …: bind: invalid argument` | 4 | The control socket path is too long: use `/tmp/<short>.sock` |
| `notification_failed … bad certificate` on the server | | The listener of `client subscribe` refuses the server's certificate: its `--ca` must hold the server's CA |
| `cancelling subscription …: its notifications fail` | | The server cannot reach `--notify-url`; on a development server, `--notify-loopback` allows `127.0.0.1` |

`--debug` shows the causes of an error.

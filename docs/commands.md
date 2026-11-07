# Command reference

Every command of ieee20305ctl with its flags, as `--help` prints them. Run
`ieee20305ctl <command> --help` for the description and examples of a
command. Every command also has `-h`, `--help` and the global `--debug`
(show the causes of an error, and the requests and answers of the library,
on stderr).

Flags marked `[env: …]` can come from that environment variable; a flag given
on the command line wins. Exit codes: [exitcodes.md](exitcodes.md).

## `cert generate`

```text
ieee20305ctl cert generate [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--devices` | int | Number of device certificates (default 1) |
| `--force` | bool | Overwrite existing files |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--hmn` | bool | Give the device certificates a HardwareModuleName (with their name as serial number), as IEEE 2030.5 device certificates have |
| `--org` | string | Organization of the certificates' subjects (default "IEEE 2030.5 Test") |
| `--out` | string | Directory of the files (default ".") |
| `--server-name` | strings | Host names and IP addresses of the server certificate (default [localhost,127.0.0.1]) |

## `cert inspect`

```text
ieee20305ctl cert inspect <certificate> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |

## `cert lfdi`

```text
ieee20305ctl cert lfdi <certificate> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |

## `cert verify`

```text
ieee20305ctl cert verify <certificate> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates (PEM) to verify against |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--name` | string | Host name or IP address the certificate must be for (a server's) |
| `--usage` | string | Extended key usage to require: client (a device), server, or any (default "client") |

## `client connection-point`

```text
ieee20305ctl client connection-point [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--set` | string | Put this connection point ID (the NMI in Australia) before reading it |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client controls`

```text
ieee20305ctl client controls [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--active` | bool | Only the controls the server reports active |
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--program` | string | Only the controls of this program: its href or description |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client dcap`

```text
ieee20305ctl client dcap [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client delete`

```text
ieee20305ctl client delete <href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |
| `--yes` | bool | Confirm the deletion |

## `client der put`

```text
ieee20305ctl client der put [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--availability` | string | File of the DERAvailability (XML) |
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--capability` | string | File of the DERCapability (XML) |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--der` | string | Href of the DER (default: the first of the device) |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--settings` | string | File of the DERSettings (XML) |
| `--status` | string | File of the DERStatus (XML) |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client der show`

```text
ieee20305ctl client der show [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |
| `--xml` | bool | Include each resource the device has put, as XML |

## `client discover`

```text
ieee20305ctl client discover [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--subtype` | strings | Only servers of these function-set subtypes, such as derp or edev |
| `--wait` | duration | How long to listen for answers (default 5s) |

## `client flow-reservation`

```text
ieee20305ctl client flow-reservation [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--duration` | duration | Length of the interval in which the energy is wanted (default 2h0m0s) |
| `--duration-requested` | duration | How long the device needs to draw it, within the interval (0: unset) |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--energy` | int | Energy requested, in Wh (negative: to the grid) |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--power` | int | Power requested, in W |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--start` | duration | Start of the interval, from now |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client get`

```text
ieee20305ctl client get <href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client log-event`

```text
ieee20305ctl client log-event [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--code` | uint8 | logEventCode: the severity (0 informational, 1 warning, 2 error, ...) |
| `--details` | string | details, at most 32 characters |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--function-set` | uint8 | functionSet the event is about |
| `--id` | uint16 | logEventID: the event, in the numbering of --pen |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--pen` | uint32 | logEventPEN: the IANA Private Enterprise Number that numbers the event (0: IEEE 2030.5) |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--profile-id` | uint8 | profileID of the function set |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client post`

```text
ieee20305ctl client post <list-href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--file` | string | File of the document (XML), checked against the schema before it is sent |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client pricing`

```text
ieee20305ctl client pricing [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client programs`

```text
ieee20305ctl client programs [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client put`

```text
ieee20305ctl client put <href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--file` | string | File of the document (XML), checked against the schema before it is sent |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client register`

```text
ieee20305ctl client register [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--no-create` | bool | Only look the EndDevice up; do not post one when the server has none |
| `--pin` | uint32 | PIN the device was given out of band, checked against its Registration (0: do not check) [env: IEEE20305CTL_PIN] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client report`

```text
ieee20305ctl client report [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--description` | string | Without --file: description of the MirrorUsagePoint (default "ieee20305ctl") |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--file` | string | File of the MirrorUsagePoint to post (XML), with its MirrorMeterReadings |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--power` | int | Without --file: post the real power of the device, in watts |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client respond`

```text
ieee20305ctl client respond <control-href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--status` | string | Response status: received, started, completed, opt-out, opt-in, cancelled, superseded, or its number (default "received") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--to` | string | Post to this ResponseList instead of the control's replyTo |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client run`

```text
ieee20305ctl client run [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--duration` | duration | Stop after this long (0: until interrupted) |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--notify-allow-anonymous` | bool | Also take notifications from a peer that presents no certificate, as some servers (GridAPPS-D) post them; anyone who reaches the listener can then post notifications |
| `--notify-listen` | string | Serve a notification listener on this address and subscribe to the device's programs (needs --notify-url when the address is not reachable as is) |
| `--notify-url` | string | URL the server posts notifications to |
| `--pin` | uint32 | PIN the device was given out of band; the agent stops when the Registration has another (0: do not check) [env: IEEE20305CTL_PIN] |
| `--poll-rate` | duration | Poll at this rate instead of the server's pollRate |
| `--polls` | bool | Write a record for each poll too, not only for failed ones |
| `--post-rate` | duration | Rate of the metering reports of --report-power (default 1m0s) |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--report-power` | int | Report this real power (W) through a metering mirror at --post-rate (0: no metering) |
| `--subscribed-poll-rate` | duration | Poll rate while subscribed (default 15m0s) |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client subscribe`

```text
ieee20305ctl client subscribe <resource-href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--count` | int | Stop after this many notifications (0: until interrupted or --duration) |
| `--duration` | duration | Stop after this long (0: until interrupted or --count) |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--keep` | bool | Leave the subscription on the server when the command ends |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--limit` | uint32 | Most items of a list a notification carries (default 10) |
| `--listen` | string | Address the notification listener binds (default "0.0.0.0:8443") |
| `--listen-allow-anonymous` | bool | Also take notifications from a peer that presents no certificate, as some servers (GridAPPS-D) post them; anyone who reaches the listener can then post notifications |
| `--listen-cert` | string | Certificate of the listener (default: the device certificate) |
| `--listen-key` | string | Key of the listener (default: the device key) |
| `--notify-url` | string | URL the server posts notifications to (default: https://<--listen>/notify, which needs a --listen address the server can reach) |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client time`

```text
ieee20305ctl client time [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `client walk`

```text
ieee20305ctl client walk [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--ca` | string | CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D) [env: IEEE20305CTL_CCM8] |
| `--cert` | string | Device certificate (PEM) [env: IEEE20305CTL_CERT] |
| `--depth` | int | Number of links to follow from the root (default 3) |
| `--edition` | string | Edition the device speaks: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--from` | string | Href to start from (default: the DeviceCapability) |
| `--key` | string | Private key of the device certificate (PEM) [env: IEEE20305CTL_KEY] |
| `--profile` | string | Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |
| `--timeout` | duration | Time limit of each request [env: IEEE20305CTL_TIMEOUT] (default 30s) |
| `--url` | string | URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap) [env: IEEE20305CTL_URL] |

## `server cancel`

```text
ieee20305ctl server cancel <control-href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--timeout` | duration | Time limit of the operation (default 30s) |
| `--yes` | bool | Confirm: devices stop the control |

## `server devices`

```text
ieee20305ctl server devices [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--timeout` | duration | Time limit of the operation (default 30s) |

## `server generate-config`

```text
ieee20305ctl server generate-config [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--force` | bool | Overwrite --output |
| `--output` | string | Write to this file instead of stdout |

## `server get`

```text
ieee20305ctl server get <href> [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--timeout` | duration | Time limit of the operation (default 30s) |

## `server issue-control`

```text
ieee20305ctl server issue-control [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--description` | string | Description of the control |
| `--duration` | duration | Length of the control (default 1h0m0s) |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--no-responses` | bool | Ask for no Responses |
| `--program` | string | Program: its href or description (default: the only one) |
| `--set` | strings | Controls as name=value: opModConnect=false, opModMaxLimW=50 (percent), opModExpLimW=1500 (W), ... |
| `--start` | duration | Start, from now |
| `--timeout` | duration | Time limit of the operation (default 30s) |
| `--yes` | bool | Confirm: devices act on the control |

## `server provision`

```text
ieee20305ctl server provision [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--cert` | string | Certificate of the device (PEM) |
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--edition` | string | Edition of the device: 2023 or 2018 (default: the server's) |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--lfdi` | string | LFDI of the device, instead of --cert |
| `--pin` | uint32 | Registration PIN |
| `--role` | string | device, aggregator or operator (default "device") |
| `--timeout` | duration | Time limit of the operation (default 30s) |
| `--unregistered` | bool | Leave the device unregistered (server register later) |

## `server register`

```text
ieee20305ctl server register [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--lfdi` | string | LFDI of the device |
| `--pin` | uint32 | Registration PIN |
| `--timeout` | duration | Time limit of the operation (default 30s) |

## `server set-default`

```text
ieee20305ctl server set-default [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--control-socket` | string | Unix socket of the running server start [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--program` | string | Program: its href or description (default: the only one) |
| `--set` | strings | Controls as name=value, as issue-control takes them |
| `--timeout` | duration | Time limit of the operation (default 30s) |
| `--yes` | bool | Confirm: devices apply it when no control does |

## `server start`

```text
ieee20305ctl server start [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--advertise` | bool | Advertise the server with DNS-SD (_smartenergy._tcp) on the local network |
| `--ca` | string | CA certificates of the devices (PEM) [env: IEEE20305CTL_CA] |
| `--ccm8` | bool | Serve TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first, then AES-128-GCM (for GridAPPS-D and EPRI clients) |
| `--cert` | string | Server certificate (PEM) [env: IEEE20305CTL_SERVER_CERT] |
| `--config` | string | Seed file (YAML or JSON): devices, programs, tariffs; see server generate-config [env: IEEE20305CTL_SERVER_CONFIG] |
| `--control-socket` | string | Unix socket of the operator commands (none when empty) [env: IEEE20305CTL_CONTROL_SOCKET] |
| `--duration` | duration | Stop after this long (0: until interrupted) |
| `--edition` | string | Edition of devices the seed does not give one: 2023 or 2018 [env: IEEE20305CTL_EDITION] (default "2023") |
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--in-band` | bool | Let unknown devices post their EndDevice (in-band registration) |
| `--key` | string | Private key of the server certificate (PEM) [env: IEEE20305CTL_SERVER_KEY] |
| `--listen` | string | Address to listen on [env: IEEE20305CTL_LISTEN] (default "0.0.0.0:8443") |
| `--log-requests` | bool | Write a record for every request |
| `--notify-loopback` | bool | Deliver notifications to loopback and link-local addresses too (for local tests) |
| `--profile` | string | Profile: ieee20305 or csipaus (CSIP-AUS documents, connection points, IEEE 2030.5-2018) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |

## `validate`

```text
ieee20305ctl validate <file>... [flags]
```

| Flag | Type | Meaning |
|------|------|---------|
| `--format` | string | Output format: text, json or table [env: IEEE20305CTL_FORMAT] (default "text") |
| `--profile` | string | Schema to validate against: ieee20305 (IEEE 2030.5-2023) or csipaus (CSIP-AUS v1.3) [env: IEEE20305CTL_PROFILE] (default "ieee20305") |

## `version`

```text
ieee20305ctl version [flags]
```

No flags of its own.


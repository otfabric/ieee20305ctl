// SPDX-License-Identifier: MIT

// Package site is the server of ieee20305ctl: a go-ieee20305 server seeded
// from a configuration file (docs/server-config.md), and the operations an
// operator runs on it while it serves, through its control socket.
package site

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Seed is the configuration file of server start, in YAML or JSON.
type Seed struct {
	// PollRate is the pollRate the server gives devices, in seconds.
	PollRate uint32 `yaml:"poll_rate" json:"poll_rate"`
	// Devices are the devices the server is told of.
	Devices []DeviceSeed `yaml:"devices" json:"devices"`
	// Programs are the DER programs every device of role device is
	// assigned, with their default controls and controls.
	Programs []ProgramSeed `yaml:"programs" json:"programs"`
	// Tariffs are the tariffs every device of role device is assigned.
	Tariffs []TariffSeed `yaml:"tariffs" json:"tariffs"`
}

// DeviceSeed is a device of the seed.
type DeviceSeed struct {
	// Cert is the device's certificate (PEM), or LFDI its LFDI.
	Cert string `yaml:"cert" json:"cert"`
	LFDI string `yaml:"lfdi" json:"lfdi"`
	// PIN is the registration PIN, given to the device out of band.
	PIN uint32 `yaml:"pin" json:"pin"`
	// Edition is 2023 (the default) or 2018.
	Edition string `yaml:"edition" json:"edition"`
	// Role is device (the default), aggregator or operator.
	Role string `yaml:"role" json:"role"`
	// ManagedBy is the LFDI of the aggregator that manages the device.
	ManagedBy string `yaml:"managed_by" json:"managed_by"`
	// Unregistered leaves the device unregistered until server register.
	Unregistered bool `yaml:"unregistered" json:"unregistered"`
	// DeviceCategory is the deviceCategory of its EndDevice, in hex.
	DeviceCategory string `yaml:"device_category" json:"device_category"`
}

// Settings are the controls of a DERControlBase by schema name, as the
// --set flags of server issue-control take them: opModMaxLimW: 50.
type Settings map[string]any

// Pairs returns the settings as name=value pairs, in name order.
func (s Settings) Pairs() []string {
	var out []string
	for k, v := range s {
		out = append(out, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(out)
	return out
}

// ProgramSeed is a DER program of the seed.
type ProgramSeed struct {
	Description string        `yaml:"description" json:"description"`
	Primacy     uint8         `yaml:"primacy" json:"primacy"`
	MRID        string        `yaml:"mrid" json:"mrid"`
	Default     Settings      `yaml:"default" json:"default"`
	Controls    []ControlSeed `yaml:"controls" json:"controls"`
}

// ControlSeed is a DER control of the seed, which starts Start after the
// server does.
type ControlSeed struct {
	Description string   `yaml:"description" json:"description"`
	MRID        string   `yaml:"mrid" json:"mrid"`
	Set         Settings `yaml:"set" json:"set"`
	Start       Duration `yaml:"start" json:"start"`
	Duration    Duration `yaml:"duration" json:"duration"`
	// NoResponses leaves out replyTo and responseRequired.
	NoResponses bool `yaml:"no_responses" json:"no_responses"`
}

// TariffSeed is a tariff profile of the seed.
type TariffSeed struct {
	Description     string          `yaml:"description" json:"description"`
	MRID            string          `yaml:"mrid" json:"mrid"`
	Currency        uint16          `yaml:"currency" json:"currency"`
	PriceMultiplier int8            `yaml:"price_multiplier" json:"price_multiplier"`
	Primacy         uint8           `yaml:"primacy" json:"primacy"`
	RateCode        string          `yaml:"rate_code" json:"rate_code"`
	RateComponents  []RateComponent `yaml:"rate_components" json:"rate_components"`
}

// RateComponent is a rate component of a tariff of the seed.
type RateComponent struct {
	Description   string         `yaml:"description" json:"description"`
	MRID          string         `yaml:"mrid" json:"mrid"`
	RoleFlags     string         `yaml:"role_flags" json:"role_flags"`
	FlowDirection uint8          `yaml:"flow_direction" json:"flow_direction"`
	Intervals     []IntervalSeed `yaml:"intervals" json:"intervals"`
}

// IntervalSeed is a time tariff interval, with the price of its one
// consumption tariff interval in the tariff's multiplier.
type IntervalSeed struct {
	Description string   `yaml:"description" json:"description"`
	MRID        string   `yaml:"mrid" json:"mrid"`
	Start       Duration `yaml:"start" json:"start"`
	Duration    Duration `yaml:"duration" json:"duration"`
	TOUTier     uint8    `yaml:"tou_tier" json:"tou_tier"`
	Price       int32    `yaml:"price" json:"price"`
}

// Duration is a time.Duration written as "90s", "1h30m" or a number of
// seconds.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	return d.parse(s)
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error { return d.parse(strings.Trim(string(b), `"`)) }

func (d *Duration) parse(s string) error {
	if v, err := time.ParseDuration(s); err == nil {
		*d = Duration(v)
		return nil
	}
	var secs int64
	if _, err := fmt.Sscan(s, &secs); err != nil {
		return fmt.Errorf("%q is not a duration such as 90s or 1h", s)
	}
	if limit := int64(math.MaxInt64 / time.Second); secs > limit || secs < -limit {
		return fmt.Errorf("%q seconds is too long a duration", s)
	}
	*d = Duration(time.Duration(secs) * time.Second)
	return nil
}

// LoadSeed reads a seed file.
func LoadSeed(file string) (*Seed, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s, err := ParseSeed(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return s, nil
}

// ParseSeed decodes and validates the content of a seed file.
func ParseSeed(data []byte) (*Seed, error) {
	var s Seed
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Validate checks what the seed must have.
func (s *Seed) Validate() error {
	for i, d := range s.Devices {
		if (d.Cert == "") == (d.LFDI == "") {
			return fmt.Errorf("devices[%d]: give cert or lfdi", i)
		}
		switch d.Role {
		case "", "device", "aggregator", "operator":
		default:
			return fmt.Errorf("devices[%d]: role %q: device, aggregator or operator", i, d.Role)
		}
		switch d.Edition {
		case "", "2018", "2023":
		default:
			return fmt.Errorf("devices[%d]: edition %q: 2018 or 2023", i, d.Edition)
		}
	}
	for i, p := range s.Programs {
		if p.Description == "" {
			return fmt.Errorf("programs[%d]: a description is required: operators name programs by it", i)
		}
		for j, c := range p.Controls {
			if c.Duration <= 0 {
				return fmt.Errorf("programs[%d].controls[%d]: a positive duration is required", i, j)
			}
		}
	}
	return nil
}

// mrid parses a hex mRID, or derives one from a name when it is empty.
func mrid(s, name string) ([]byte, error) {
	if s != "" {
		b, err := hex.DecodeString(s)
		if err != nil || len(b) == 0 || len(b) > 16 {
			return nil, fmt.Errorf("mrid %q: up to 32 hexadecimal digits", s)
		}
		return b, nil
	}
	var b [16]byte
	h := fnv(name)
	for i := range b {
		b[i] = byte(h >> (8 * (i % 8)))
		if i == 7 {
			h = fnv(fmt.Sprint(h))
		}
	}
	return b[:], nil
}

func fnv(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// Example is the commented seed of server generate-config.
const Example = `# ieee20305ctl server seed: the devices the server knows, and the programs and
# tariffs it assigns them. YAML or JSON; durations as 90s, 15m, 1h, or seconds.
# See docs/server-config.md.

# pollRate the server gives devices, in seconds.
poll_rate: 60

devices:
  # A device by its certificate (or "lfdi: 3E4F45AB..."), with the PIN it
  # was given out of band. edition: 2023 (default) or 2018; role: device
  # (default), aggregator or operator.
  - cert: device.crt
    pin: 111115

programs:
  # Assigned to every device of role device. Lower primacy wins.
  - description: Utility program
    primacy: 1
    # What applies when no control does. Settings by schema name:
    # opModConnect/opModEnergize true|false, opModMaxLimW and opModFixedW in
    # percent, opModTargetW and the CSIP-AUS opModExpLimW, opModImpLimW,
    # opModGenLimW, opModLoadLimW in watts.
    default:
      opModConnect: true
      opModMaxLimW: 100
    controls:
      # start: from when the server starts; the device answers each control
      # with Responses unless no_responses: true.
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
    currency: 36            # ISO 4217 number: 36 AUD, 840 USD, 978 EUR
    price_multiplier: -4    # prices in 1/10000 of the currency per kWh
    primacy: 1
    rate_code: TOU1
    rate_components:
      - description: Import
        role_flags: "0001"
        flow_direction: 1   # forward: from the grid
        intervals:
          - description: Off-peak
            start: 0s
            duration: 1h
            price: 1500     # 0.15 per kWh
          - description: Peak
            start: 1h
            duration: 1h
            price: 4500
`

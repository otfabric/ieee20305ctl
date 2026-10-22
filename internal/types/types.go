// SPDX-License-Identifier: MIT

// Package types provides CLI-specific type aliases and wrappers for ieee20305ctl.
package types

// LFDI represents a Long-Form Device Identifier (40 hex characters / 160 bits).
type LFDI string

// SFDI represents a Short-Form Device Identifier (5 decimal digits derived from LFDI).
type SFDI uint64

// DeviceInfo holds summary information about an IEEE 2030.5 EndDevice.
type DeviceInfo struct {
	// LFDI is the device's long-form identifier.
	LFDI LFDI `json:"lfdi" yaml:"lfdi"`

	// SFDI is the device's short-form identifier.
	SFDI SFDI `json:"sfdi" yaml:"sfdi"`

	// Href is the device's resource URI on the server.
	Href string `json:"href,omitempty" yaml:"href,omitempty"`
}

// ServerInfo holds summary information about an IEEE 2030.5 server.
type ServerInfo struct {
	// Host is the server address.
	Host string `json:"host" yaml:"host"`

	// DeviceCapabilityHref is the root resource URI.
	DeviceCapabilityHref string `json:"deviceCapabilityHref" yaml:"deviceCapabilityHref"`
}

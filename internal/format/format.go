// SPDX-License-Identifier: MIT

// Package format provides output formatting support for ieee20305ctl.
package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Format represents an output format type.
type Format string

// Supported output formats.
const (
	Text  Format = "text"
	Table Format = "table"
	JSON  Format = "json"
)

// String returns the format as a string.
func (f Format) String() string {
	return string(f)
}

// Valid reports whether f is a supported format.
func (f Format) Valid() bool {
	switch f {
	case Text, Table, JSON:
		return true
	default:
		return false
	}
}

// Parse parses a format string, returning an error if it is not supported.
func Parse(s string) (Format, error) {
	f := Format(strings.ToLower(strings.TrimSpace(s)))
	if !f.Valid() {
		return "", fmt.Errorf("unsupported output format %q (use text, table, or json)", s)
	}
	return f, nil
}

// AddFlag adds an --output flag to the given command and returns a pointer to
// the format value. The default is Text.
func AddFlag(cmd *cobra.Command) *string {
	var output string
	cmd.Flags().StringVarP(&output, "output", "o", string(Text),
		"output format: text, table, json")
	return &output
}

// envelope is the JSON document of every command; see docs/json-output.md.
type envelope struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

// WriteJSON writes data as a successful JSON result.
func WriteJSON(w io.Writer, data any) error {
	return writeEnvelope(w, envelope{Status: "ok", Data: data})
}

// WriteJSONError writes err as a failed JSON result.
func WriteJSONError(w io.Writer, err error) error {
	return writeEnvelope(w, envelope{Status: "error", Error: err.Error()})
}

func writeEnvelope(w io.Writer, e envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(e)
}

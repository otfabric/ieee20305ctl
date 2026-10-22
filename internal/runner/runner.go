// SPDX-License-Identifier: MIT

// Package runner provides an invocation context for structured CLI output.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/otfabric/ieee20305ctl/internal/format"
)

// Invocation holds the runtime context for a single CLI command execution.
type Invocation struct {
	// Ctx is the command context, typically wired with signal handling.
	Ctx context.Context

	// Stdout is the writer for normal output.
	Stdout io.Writer

	// Stderr is the writer for diagnostic output.
	Stderr io.Writer

	// Format is the selected output format.
	Format format.Format

	// Debug enables verbose diagnostic output.
	Debug bool
}

// New creates a new Invocation with sensible defaults.
func New(ctx context.Context) *Invocation {
	return &Invocation{
		Ctx:    ctx,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Format: format.Text,
	}
}

// WriteJSON encodes v as indented JSON to the invocation's stdout.
func (inv *Invocation) WriteJSON(v any) error {
	enc := json.NewEncoder(inv.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Debugf writes a formatted debug message to stderr when debug mode is enabled.
func (inv *Invocation) Debugf(msg string, args ...any) {
	if inv.Debug {
		_, _ = fmt.Fprintf(inv.Stderr, "DEBUG: "+msg+"\n", args...)
	}
}

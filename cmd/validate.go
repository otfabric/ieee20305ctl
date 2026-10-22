// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/spf13/cobra"
)

// ValidationResult is the outcome of validating one document.
type ValidationResult struct {
	File    string `json:"file"`
	Element string `json:"element,omitempty"`
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
}

// profiles maps --profile to a decoder and the element names it knows.
var profiles = map[string]struct {
	decode      func([]byte) (any, error)
	elementName func(any) (xml.Name, bool)
}{
	"ieee20305": {ieee20305.Decode, ieee20305.ElementName},
	"csipaus": {csipaus.Decode, func(v any) (xml.Name, bool) {
		if n, ok := csipaus.ElementName(v); ok {
			return n, true
		}
		return ieee20305.ElementName(v)
	}},
}

func newValidateCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "validate [file...]",
		Short: "Validate IEEE 2030.5 XML documents offline",
		Long: `Decode IEEE 2030.5 XML documents and check them against the constraints of
the schema: required elements and their cardinality, string and binary
lengths, numeric ranges and patterns. With --profile csipaus, documents are
read as CSIP-AUS v1.3, whose elements in the CSIP-AUS namespace are checked
as well.

A required element or attribute that is absent decodes to its zero value and
is not reported; validate against the official XSD (for example with
xmllint) for that.

The exit status is 5 when a document is invalid.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			p, ok := profiles[profile]
			if !ok {
				return errs.WithCode(errs.ExitUsage, fmt.Errorf("unknown profile %q (use ieee20305 or csipaus)", profile))
			}
			results := make([]ValidationResult, 0, len(args))
			invalid := 0
			for _, path := range args {
				r := validateFile(path, p.decode, p.elementName)
				if !r.Valid {
					invalid++
				}
				results = append(results, r)
			}
			if invalid > 0 {
				err = errs.WithCode(errs.ExitInvalid, fmt.Errorf("%d of %d documents are invalid", invalid, len(results)))
			}
			if f == format.JSON {
				if werr := format.WriteJSONResult(cmd.OutOrStdout(), results, err); werr != nil {
					return werr
				}
				return err
			}
			if werr := printValidation(cmd.OutOrStdout(), results); werr != nil {
				return werr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "ieee20305", "schema to validate against: ieee20305 (IEEE 2030.5-2023) or csipaus (CSIP-AUS v1.3)")
	format.AddFlag(cmd)
	return cmd
}

func validateFile(path string, decode func([]byte) (any, error), elementName func(any) (xml.Name, bool)) ValidationResult {
	r := ValidationResult{File: path}
	data, err := os.ReadFile(path)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	v, err := decode(data)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	if name, ok := elementName(v); ok {
		r.Element = name.Local
	}
	if err := v.(interface{ Validate() error }).Validate(); err != nil {
		r.Error = err.Error()
		return r
	}
	r.Valid = true
	return r
}

func printValidation(w io.Writer, results []ValidationResult) error {
	for _, r := range results {
		var err error
		if r.Valid {
			_, err = fmt.Fprintf(w, "%s: valid %s\n", r.File, r.Element)
		} else {
			_, err = fmt.Fprintf(w, "%s: invalid: %s\n", r.File, r.Error)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

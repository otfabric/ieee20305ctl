// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/runner"
)

// ValidationResult is the outcome of validating one document.
type ValidationResult struct {
	File    string `json:"file"`
	Element string `json:"element,omitempty"`
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
}

// Validation is the result of validate.
type Validation struct {
	Profile   string             `json:"profile"`
	Documents []ValidationResult `json:"documents"`
	Invalid   int                `json:"invalid"`
}

func (v *Validation) MarshalTextOutput() (string, error) {
	var b strings.Builder
	for _, r := range v.Documents {
		if r.Valid {
			fmt.Fprintf(&b, "%s: valid %s\n", r.File, r.Element)
		} else {
			fmt.Fprintf(&b, "%s: INVALID: %s\n", r.File, r.Error)
		}
	}
	if v.Invalid > 0 {
		fmt.Fprintf(&b, "%d of %d documents are invalid\n", v.Invalid, len(v.Documents))
	}
	return b.String(), nil
}

func (v *Validation) TableHeaders() []string { return []string{"FILE", "ELEMENT", "VALID", "ERROR"} }
func (v *Validation) TableRows() [][]string {
	rows := [][]string{}
	for _, r := range v.Documents {
		rows = append(rows, []string{r.File, r.Element, strconv.FormatBool(r.Valid), r.Error})
	}
	return rows
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

type validateConfig struct {
	Profile string `flag:"profile" env:"IEEE20305CTL_PROFILE" desc:"Schema to validate against: ieee20305 (IEEE 2030.5-2023) or csipaus (CSIP-AUS v1.3)"`
	Format  string `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newValidateCmd() *cobra.Command {
	c := validateConfig{Profile: "ieee20305", Format: "text"}
	cmd := &cobra.Command{
		Use:   "validate <file>...",
		Short: "Validate IEEE 2030.5 XML documents against the schema",
		Long: `Decode each XML document and check it against the IEEE 2030.5-2023 schema (or CSIP-AUS
v1.3 with --profile csipaus): its element, the facets and cardinalities of every value.
The result lists every file; when one is invalid the result is still written, and the
exit code is 3.`,
		Example: `  ieee20305ctl validate dercontrol.xml
  ieee20305ctl validate --profile csipaus *.xml --format json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.RunFormattedChecked(cmd, c.Format, func() error {
				if _, ok := profiles[c.Profile]; !ok {
					return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--profile %q: ieee20305 or csipaus", c.Profile), nil)
				}
				return nil
			}, func(context.Context) (any, error) {
				p := profiles[c.Profile]
				res := &Validation{Profile: c.Profile, Documents: []ValidationResult{}}
				for _, f := range args {
					r := validateFile(f, p.decode, p.elementName)
					if !r.Valid {
						res.Invalid++
					}
					res.Documents = append(res.Documents, r)
				}
				return res, nil
			}, runner.WithSuccessExit(func(v any) int {
				if v.(*Validation).Invalid > 0 {
					return errs.ExitInvalidInput
				}
				return errs.ExitOK
			}))
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
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
	if n, ok := elementName(v); ok {
		r.Element = n.Local
	}
	if val, ok := v.(interface{ Validate() error }); ok {
		if err := val.Validate(); err != nil {
			r.Error = err.Error()
			return r
		}
	}
	r.Valid = true
	return r
}

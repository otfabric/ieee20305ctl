// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-ieee20305/discovery"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/runner"
)

// Service is a record of client discover: a server found with DNS-SD.
type Service struct {
	Instance string   `json:"instance"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Addrs    []string `json:"addresses"`
	URL      string   `json:"url,omitempty"`
	Subtypes []string `json:"subtypes,omitempty"`
	Removed  bool     `json:"removed,omitempty"`
}

func (s *Service) MarshalTextOutput() (string, error) {
	state := ""
	if s.Removed {
		state = " (gone)"
	}
	return fmt.Sprintf("%s%s\n  %s:%d %s\n  %s", s.Instance, state, s.Host, s.Port, strings.Join(s.Addrs, " "), s.URL), nil
}

func (s *Service) TableHeaders() []string {
	return []string{"INSTANCE", "HOST", "PORT", "URL", "REMOVED"}
}
func (s *Service) TableRows() [][]string {
	return [][]string{{s.Instance, s.Host, strconv.Itoa(s.Port), s.URL, strconv.FormatBool(s.Removed)}}
}

type discoverConfig struct {
	Wait     time.Duration `flag:"wait" desc:"How long to listen for answers"`
	Subtypes []string      `flag:"subtype" desc:"Only servers of these function-set subtypes, such as derp or edev"`
	Format   string        `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newDiscoverCmd() *cobra.Command {
	c := discoverConfig{Wait: 5 * time.Second, Format: "text"}
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find IEEE 2030.5 servers on the local network with DNS-SD",
		Long: `Browse the local network with multicast DNS for IEEE 2030.5 servers (_smartenergy._tcp,
or the function-set subtypes of --subtype) for --wait, and write one record per server
found, with the URL of its DeviceCapability. Needs a network with multicast: it finds
nothing across routers or in most container networks.`,
		Example: `  ieee20305ctl client discover --wait 10s
  ieee20305ctl client discover --subtype derp --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.RunStream(cmd, c.Format, func() error {
				if c.Wait <= 0 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--wait must be positive", nil)
				}
				return nil
			}, func(ctx context.Context, out *format.StreamWriter) (int, error) {
				ctx, cancel := context.WithTimeout(ctx, c.Wait)
				defer cancel()
				found, err := discovery.Browse(ctx, discovery.BrowseOptions{Subtypes: c.Subtypes})
				if err != nil {
					return 0, errs.New(errs.KindTransport, errs.CodeListenFailed, "DNS-SD: "+err.Error(), err)
				}
				for s := range found {
					rec := &Service{Instance: s.Instance, Host: s.Host, Port: s.Port, Addrs: []string{}, Subtypes: s.Subtypes, Removed: s.Removed}
					for _, a := range s.Addrs {
						rec.Addrs = append(rec.Addrs, a.String())
					}
					if u, err := s.DcapURL(); err == nil {
						rec.URL = u
					}
					if err := out.Write(rec); err != nil {
						return 0, errs.Output(errs.CodeOutputWriteFailed, err)
					}
				}
				return errs.ExitOK, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

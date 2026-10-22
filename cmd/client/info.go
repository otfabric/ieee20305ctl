// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

func newDcapCmd() *cobra.Command {
	c := newCommon()
	cmd := &cobra.Command{
		Use:   "dcap",
		Short: "Read the server's DeviceCapability and the function sets it links",
		Long: `Read the DeviceCapability of the server, the root of its resources, and list the
links it has: the function sets the server offers, with the number of items of each list.`,
		Example: `  ieee20305ctl client dcap --url https://utility.example/dcap --cert device.crt --key device.key --ca ca.crt`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				dcap, err := d.Client.DeviceCapability(ctx)
				if err != nil {
					return nil, err
				}
				res := &types.Capability{URL: d.URL, PollRate: uint32(dcap.GetPollRate()), Links: []types.Link{}}
				for _, l := range ieee20305.Links(dcap) {
					link := types.Link{Name: l.Element, Href: l.Href}
					if l.All != nil {
						n := uint32(*l.All)
						link.All = &n
					}
					res.Links = append(res.Links, link)
				}
				return res, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func newTimeCmd() *cobra.Command {
	c := newCommon()
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Read the server's clock and how far the local clock is from it",
		Long: `Read the Time resource of the server. Devices schedule events by the server's clock:
the offset says how far the local clock is from it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				ts, err := d.Client.Time(ctx)
				if err != nil {
					return nil, err
				}
				now := time.Now().UTC().Truncate(time.Second)
				return &types.TimeResult{ServerTime: now.Add(ts.Offset).Truncate(time.Second), LocalTime: now,
					OffsetMS: ts.Offset.Milliseconds(), TZOffsetS: int32(ts.Time.TzOffset), DSTOffsetS: int32(ts.Time.DstOffset),
					Quality: uint8(ts.Time.Quality)}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type registerConfig struct {
	Common
	PIN      uint32 `flag:"pin" env:"IEEE20305CTL_PIN" desc:"PIN the device was given out of band, checked against its Registration (0: do not check)"`
	NoCreate bool   `flag:"no-create" desc:"Only look the EndDevice up; do not post one when the server has none"`
}

func newRegisterCmd() *cobra.Command {
	c := registerConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Find or create the device's EndDevice, and check the PIN of its Registration",
		Long: `Find the EndDevice of the device in the server's EndDeviceList, by the SFDI and LFDI of
its certificate, or post one when there is none (unless --no-create). Then read its
Registration and, with --pin, check its PIN: a device must not trust a server whose PIN
is not the one it was given out of band. A PIN that does not match exits with 6.`,
		Example: `  ieee20305ctl client register --pin 111115
  ieee20305ctl client register --no-create --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c.Common, nil, true, func(ctx context.Context, d *conn.Device) (any, error) {
				return register(ctx, d, c.PIN, c.NoCreate)
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func register(ctx context.Context, d *conn.Device, pin uint32, noCreate bool) (any, error) {
	dcap, err := d.Client.DeviceCapability(ctx)
	if err != nil {
		return nil, err
	}
	if dcap.EndDeviceListLink == nil {
		return nil, errs.New(errs.KindProtocol, errs.CodeInvalidResponse, "the server's DeviceCapability has no EndDeviceListLink", nil)
	}
	var ed *ieee20305.EndDevice
	created := false
	if noCreate {
		if ed, err = d.Client.FindEndDevice(ctx, dcap.EndDeviceListLink.Href, d.Identity); err == nil && ed == nil {
			return nil, errs.New(errs.KindProtocol, errs.CodeNotRegistered,
				fmt.Sprintf("the server has no EndDevice for SFDI %d", d.Identity.SFDI), nil)
		}
	} else {
		ed, created, err = d.Client.Register(ctx, dcap.EndDeviceListLink.Href, d.Identity)
	}
	if err != nil {
		return nil, err
	}
	res := &types.Registration{EndDevice: ed.GetHref(), Created: created,
		LFDI: d.Identity.LFDI.String(), SFDI: d.Identity.SFDI.String()}
	reg, err := d.Client.Registration(ctx, ed)
	if errors.Is(err, ieeeclient.ErrNotRegistered) {
		return res, nil
	}
	if err != nil {
		return nil, err
	}
	res.Registered = true
	p := uint32(reg.PIN)
	res.PIN = &p
	if t := reg.DateTimeRegistered; t != 0 {
		at := unix(t)
		res.Since = &at
	}
	if pin != 0 {
		res.PINChecked = true
		ok := ieeeclient.CheckPIN(reg, pin) == nil
		res.PINMatches = &ok
		if !ok {
			return nil, errs.New(errs.KindProtocol, errs.CodePINMismatch,
				fmt.Sprintf("the Registration of %s has PIN %d, not %d: the server is not the one the device was given", ed.GetHref(), p, pin), nil).
				WithDetail(res)
		}
	}
	return res, nil
}

type getConfig struct {
	Common
}

func newGetCmd() *cobra.Command {
	c := getConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "get <href>",
		Short: "Read any resource of the server",
		Long: `Read the resource at href, a path of the server such as /edev/1/fsa, and print it as
the server sent it (indented XML). The document is decoded and re-encoded in the edition
and profile of --edition and --profile, so what the device's codec does not know is
dropped. With --format json it is {"href", "element", "xml"}.`,
		Example: `  ieee20305ctl client get /dcap
  ieee20305ctl client get /edev/1/fsa --format json | jq -r .xml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return once(cmd, &c.Common, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				doc, err := d.Client.Fetch(ctx, args[0])
				if err != nil {
					return nil, err
				}
				x, err := marshal(d, doc)
				if err != nil {
					return nil, err
				}
				return &types.Document{Href: args[0], Element: elementName(doc), XML: x}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type walkConfig struct {
	Common
	Depth int    `flag:"depth" desc:"Number of links to follow from the root"`
	From  string `flag:"from" desc:"Href to start from (default: the DeviceCapability)"`
}

func newWalkCmd() *cobra.Command {
	c := walkConfig{Common: newCommon(), Depth: 3}
	cmd := &cobra.Command{
		Use:   "walk",
		Short: "Walk the resource tree from the DeviceCapability",
		Long: `Visit the resources the server offers the device, following their links breadth first,
each once, up to --depth links from the root. One record per resource: its depth, href,
element, the link it was reached by, and for a list its number of items. With
--format json the records are JSON Lines.`,
		Example: `  ieee20305ctl client walk --depth 4
  ieee20305ctl client walk --from /edev/1 --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.RunStream(cmd, c.Format, func() error {
				if err := c.Validate(); err != nil {
					return err
				}
				if c.Depth < 0 || c.Depth > 16 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--depth: from 0 to 16", nil)
				}
				return nil
			}, func(ctx context.Context, out *format.StreamWriter) (int, error) {
				d, err := c.Connect()
				if err != nil {
					return 0, err
				}
				root := c.From
				if root == "" {
					root = d.URL
				}
				err = d.Client.Walk(ctx, root, ieeeclient.WalkOptions{MaxDepth: max(c.Depth, 0) + boolInt(c.Depth == 0)}, func(s ieeeclient.Step) error {
					if s.Depth > c.Depth {
						return nil
					}
					step := &types.WalkStep{Depth: s.Depth, Href: s.Href, Element: elementName(s.Doc), Link: s.Link.Element}
					if l, ok := s.Doc.(ieee20305.Lister); ok {
						n := len(l.ListItems())
						step.Items = &n
					}
					return out.Write(step)
				})
				if err != nil {
					return 0, conn.Classify(err, false)
				}
				return errs.ExitOK, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var _ = strings.TrimSpace

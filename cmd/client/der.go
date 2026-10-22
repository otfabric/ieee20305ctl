// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/go-ieee20305/schedule"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/ieee"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

// served is a DER program the server serves the device, with its controls.
type served struct {
	program  types.Program
	controls []types.Control
}

// derPrograms reads the DER programs of the device's FunctionSetAssignments.
func derPrograms(ctx context.Context, d *conn.Device) ([]served, error) {
	_, ed, err := endDevice(ctx, d)
	if err != nil {
		return nil, err
	}
	assignments, err := fsas(ctx, d, ed)
	if err != nil {
		return nil, err
	}
	var out []served
	seen := map[string]bool{}
	for _, fsa := range assignments {
		if fsa.DERProgramListLink == nil {
			continue
		}
		err := d.Client.Iterate(ctx, fsa.DERProgramListLink.Href, 50, func(it any) error {
			prog, ok := it.(*ieee20305.DERProgram)
			if !ok || seen[prog.GetHref()] {
				return nil
			}
			seen[prog.GetHref()] = true
			s, err := readProgram(ctx, d, prog)
			if err != nil {
				return err
			}
			out = append(out, s)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readProgram(ctx context.Context, d *conn.Device, prog *ieee20305.DERProgram) (served, error) {
	s := served{program: types.Program{Href: prog.GetHref(), MRID: hexOf(prog.MRID), Description: str(prog.Description),
		Primacy: uint8(prog.Primacy), Default: []ieee.Setting{}}}
	if prog.DefaultDERControlLink != nil {
		doc, err := d.Client.Fetch(ctx, prog.DefaultDERControlLink.Href)
		switch {
		case errors.Is(err, ieee20305.ErrNotFound):
		case err != nil:
			return s, err
		default:
			if b, ok := ieee.BaseOf(doc); ok {
				s.program.Default = ieee.Settings(b)
			}
		}
	}
	if prog.DERControlListLink != nil {
		err := d.Client.Iterate(ctx, prog.DERControlListLink.Href, 50, func(it any) error {
			if c, ok := controlOf(it, prog.GetHref()); ok {
				s.controls = append(s.controls, c)
				if c.Status == schedule.Active.String() {
					s.program.Active++
				}
			}
			return nil
		})
		if err != nil {
			return s, err
		}
	}
	s.program.Controls = len(s.controls)
	return s, nil
}

func controlOf(it any, program string) (types.Control, bool) {
	var ev *ieee20305.RandomizableEvent
	switch x := it.(type) {
	case *ieee20305.DERControl:
		ev = &x.RandomizableEvent
	case *csipaus.DERControl:
		ev = &x.RandomizableEvent
	default:
		return types.Control{}, false
	}
	b, _ := ieee.BaseOf(it)
	c := types.Control{Program: program, Href: ev.GetHref(), MRID: hexOf(ev.MRID), Description: str(ev.Description),
		Start: unix(ev.Interval.Start), DurationS: uint32(ev.Interval.Duration), Status: statusText(&ev.EventStatus),
		Settings: ieee.Settings(b)}
	if ev.ReplyTo != nil {
		c.ReplyTo = *ev.ReplyTo
	}
	if ev.ResponseRequired != nil {
		c.ResponseRequired = hexOf(*ev.ResponseRequired)
	}
	return c, true
}

func newProgramsCmd() *cobra.Command {
	c := newCommon()
	cmd := &cobra.Command{
		Use:   "programs",
		Short: "List the DER programs the server assigns the device",
		Long: `List the DER programs of the device's FunctionSetAssignments, with their primacy (lower
wins), their default control (what applies when no control does), and the number of
their controls and of those that are active.`,
		Example: `  ieee20305ctl client programs
  ieee20305ctl client programs --format table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				progs, err := derPrograms(ctx, d)
				if err != nil {
					return nil, err
				}
				res := &types.Programs{Programs: []types.Program{}}
				for _, p := range progs {
					res.Programs = append(res.Programs, p.program)
				}
				return res, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type controlsConfig struct {
	Common
	Program string `flag:"program" desc:"Only the controls of this program: its href or description"`
	Active  bool   `flag:"active" desc:"Only the controls the server reports active"`
}

func newControlsCmd() *cobra.Command {
	c := controlsConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "controls",
		Short: "List the DER controls of the device's programs",
		Long: `List the DER controls (events) of the device's DER programs: when each starts and for how
long, its status as the server keeps it (Scheduled, Active, Completed, Cancelled,
Superseded), the controls it sets, and where the device answers it (reply_to).`,
		Example: `  ieee20305ctl client controls --active
  ieee20305ctl client controls --program "CSIP utility program" --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c.Common, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				progs, err := derPrograms(ctx, d)
				if err != nil {
					return nil, err
				}
				res := &types.Controls{Controls: []types.Control{}}
				for _, p := range progs {
					if c.Program != "" && c.Program != p.program.Href && c.Program != p.program.Description {
						continue
					}
					for _, ct := range p.controls {
						if c.Active && ct.Status != schedule.Active.String() {
							continue
						}
						res.Controls = append(res.Controls, ct)
					}
				}
				return res, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// responseStatuses are the names of the Response statuses of IEEE 2030.5.
var responseStatuses = map[string]uint8{
	"received": schedule.ResponseReceived, "started": schedule.ResponseStarted, "completed": schedule.ResponseCompleted,
	"opt-out": schedule.ResponseOptOut, "opt-in": schedule.ResponseOptIn, "cancelled": schedule.ResponseCancelled,
	"superseded": schedule.ResponseSuperseded, "partial-opt-out": schedule.ResponsePartialOptOut,
	"partial-opt-in": schedule.ResponsePartialOptIn,
}

func parseStatus(s string) (uint8, error) {
	if n, ok := responseStatuses[strings.ToLower(s)]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, errs.InvalidInput(errs.CodeInvalidInput,
			fmt.Sprintf("--status %q: a status number or received, started, completed, opt-out, opt-in, cancelled, superseded", s), err)
	}
	return uint8(n), nil
}

type respondConfig struct {
	Common
	Status string `flag:"status" desc:"Response status: received, started, completed, opt-out, opt-in, cancelled, superseded, or its number"`
	To     string `flag:"to" desc:"Post to this ResponseList instead of the control's replyTo"`
}

func newRespondCmd() *cobra.Command {
	c := respondConfig{Common: newCommon(), Status: "received"}
	cmd := &cobra.Command{
		Use:   "respond <control-href>",
		Short: "Answer a DER control with a Response",
		Long: `Post a DERControlResponse for the control at control-href to its replyTo (or --to):
the status of the event at the device, as the control's responseRequired asks for.
A device agent (client run) does this by itself.`,
		Example: `  ieee20305ctl client respond /derp/1/derc/3 --status started`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var status uint8
			return once(cmd, &c.Common, func() (err error) {
				status, err = parseStatus(c.Status)
				return err
			}, true, func(ctx context.Context, d *conn.Device) (any, error) {
				doc, err := d.Client.Fetch(ctx, args[0])
				if err != nil {
					return nil, err
				}
				ct, ok := controlOf(doc, "")
				if !ok {
					return nil, errs.InvalidInput(errs.CodeUnknownResource, fmt.Sprintf("%s is a %s, not a DERControl", args[0], elementName(doc)), nil)
				}
				to := c.To
				if to == "" {
					to = ct.ReplyTo
				}
				if to == "" {
					return nil, errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("%s asks for no response (no replyTo): give --to", args[0]), nil)
				}
				mrid, _ := hexBytes(ct.MRID)
				now := ieee20305.TimeType(time.Now().Unix())
				st := ieee20305.UInt8(status)
				r := &ieee20305.DERControlResponse{Response: ieee20305.Response{CreatedDateTime: &now,
					EndDeviceLFDI: ieee20305.HexBinary160(d.Identity.LFDI[:]), Status: &st, Subject: mrid}}
				loc, err := d.Client.Post(ctx, to, r)
				if err != nil {
					return nil, err
				}
				return &types.Posted{Action: "response", Target: to, Location: loc,
					Detail: fmt.Sprintf("status %d (%s) to control %s", status, statusName(status), ct.MRID)}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func statusName(n uint8) string {
	for k, v := range responseStatuses {
		if v == n {
			return k
		}
	}
	return "status " + strconv.Itoa(int(n))
}

func newDERCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "der",
		Short: "Read and put the DER resources of the device",
		Long: `The DER resources of the device: its DERCapability (what it can do), DERSettings (how it
is set up), DERStatus (its state now) and DERAvailability (what it can deliver).`,
	}
	cmd.AddCommand(newDERShowCmd(), newDERPutCmd())
	return cmd
}

type derShowConfig struct {
	Common
	XML bool `flag:"xml" desc:"Include each resource the device has put, as XML"`
}

// derResources returns the DERs of ed with the hrefs of their resources.
func derResources(ctx context.Context, d *conn.Device, ed *ieee20305.EndDevice) ([]types.DER, error) {
	var out []types.DER
	if ed.DERListLink == nil {
		return nil, nil
	}
	err := d.Client.Iterate(ctx, ed.DERListLink.Href, 10, func(it any) error {
		der, ok := it.(*ieee20305.DER)
		if !ok {
			return nil
		}
		res := types.DER{Href: der.GetHref(), Resources: []types.DERResource{}}
		add := func(name string, href string) {
			if href != "" {
				res.Resources = append(res.Resources, types.DERResource{Name: name, Href: href})
			}
		}
		if der.DERCapabilityLink != nil {
			add("capability", der.DERCapabilityLink.Href)
		}
		if der.DERSettingsLink != nil {
			add("settings", der.DERSettingsLink.Href)
		}
		if der.DERStatusLink != nil {
			add("status", der.DERStatusLink.Href)
		}
		if der.DERAvailabilityLink != nil {
			add("availability", der.DERAvailabilityLink.Href)
		}
		out = append(out, res)
		return nil
	})
	return out, err
}

func newDERShowCmd() *cobra.Command {
	c := derShowConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the DER resources of the device and which it has put",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return once(cmd, &c.Common, nil, false, func(ctx context.Context, d *conn.Device) (any, error) {
				_, ed, err := endDevice(ctx, d)
				if err != nil {
					return nil, err
				}
				ders, err := derResources(ctx, d, ed)
				if err != nil {
					return nil, err
				}
				for i := range ders {
					for j := range ders[i].Resources {
						r := &ders[i].Resources[j]
						doc, err := d.Client.Fetch(ctx, r.Href)
						if errors.Is(err, ieee20305.ErrNotFound) {
							continue
						}
						if err != nil {
							return nil, err
						}
						r.Set = true
						if c.XML {
							if r.XML, err = marshal(d, doc); err != nil {
								return nil, err
							}
						}
					}
				}
				return &types.DERs{DERs: ders}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type derPutConfig struct {
	Common
	Capability   string `flag:"capability" desc:"File of the DERCapability (XML)"`
	Settings     string `flag:"settings" desc:"File of the DERSettings (XML)"`
	Status       string `flag:"status" desc:"File of the DERStatus (XML)"`
	Availability string `flag:"availability" desc:"File of the DERAvailability (XML)"`
	DER          string `flag:"der" desc:"Href of the DER (default: the first of the device)"`
}

func newDERPutCmd() *cobra.Command {
	c := derPutConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "put",
		Short: "Put DER resources of the device from XML files",
		Long: `Put the DERCapability, DERSettings, DERStatus and DERAvailability given as XML files to
the links of the device's DER. Each document is decoded and checked against the schema
first. With --profile csipaus the CSIP-AUS elements (doeModesSupported, doeModesEnabled)
are kept.`,
		Example: `  ieee20305ctl client der put --capability dercap.xml --settings derg.xml`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			files := map[string]*string{"capability": &c.Capability, "settings": &c.Settings, "status": &c.Status, "availability": &c.Availability}
			return once(cmd, &c.Common, func() error {
				for _, f := range files {
					if *f != "" {
						return nil
					}
				}
				return errs.InvalidInput(errs.CodeInvalidInput, "give at least one of --capability, --settings, --status, --availability", nil)
			}, true, func(ctx context.Context, d *conn.Device) (any, error) {
				_, ed, err := endDevice(ctx, d)
				if err != nil {
					return nil, err
				}
				ders, err := derResources(ctx, d, ed)
				if err != nil {
					return nil, err
				}
				var der *types.DER
				for i := range ders {
					if c.DER == "" || ders[i].Href == c.DER {
						der = &ders[i]
						break
					}
				}
				if der == nil {
					return nil, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the device has no DER "+c.DER, nil)
				}
				res := &types.Written{Action: "put", Hrefs: []string{}}
				for _, r := range der.Resources {
					file := *files[r.Name]
					if file == "" {
						continue
					}
					doc, err := readDocument(d, file)
					if err != nil {
						return nil, err
					}
					if err := d.Client.Put(ctx, r.Href, doc); err != nil {
						return nil, err
					}
					res.Hrefs = append(res.Hrefs, r.Href)
				}
				return res, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// readDocument decodes the XML document of a file in the device's codec,
// and checks it against the schema.
func readDocument(d *conn.Device, file string) (any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInputFileOpenFailed, fmt.Sprintf("%s: %v", file, err), err)
	}
	doc, err := d.Codec.Decode(data)
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInvalidDocument, fmt.Sprintf("%s: %v", file, err), err)
	}
	if v, ok := doc.(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return nil, errs.InvalidInput(errs.CodeInvalidDocument, fmt.Sprintf("%s: %v", file, err), err)
		}
	}
	return doc, nil
}

func hexBytes(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := range b {
		n, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		if err != nil {
			return nil, err
		}
		b[i] = byte(n)
	}
	return b, nil
}

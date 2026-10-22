// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/spf13/cobra"
)

// RegistrationInfo is the Registration of an EndDevice.
type RegistrationInfo struct {
	Href        string    `json:"href"`
	PIN         uint32    `json:"pin"`
	PINVerified bool      `json:"pinVerified"`
	Registered  time.Time `json:"registered"`
	PollRate    uint32    `json:"pollRate"`
}

// RegisterResult is the result of client register.
type RegisterResult struct {
	LFDI         string            `json:"lfdi"`
	SFDI         uint64            `json:"sfdi"`
	EndDevice    string            `json:"endDevice"`
	Created      bool              `json:"created"`
	Registration *RegistrationInfo `json:"registration,omitempty"`
}

type registerOptions struct {
	pin      uint32
	noCreate bool
}

func newRegisterCmd(opts *options) *cobra.Command {
	ro := &registerOptions{}
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register an EndDevice with the server",
		Long: `Make sure the server has an EndDevice for this device, identified by the
SFDI and LFDI of its certificate: look it up in the server's EndDeviceList,
and create it there when it is absent (unless --no-create). Then read its
Registration and, with --pin, check that the server registered the PIN the
device was given out of band, as a device must before it trusts the server.

A device the server has not registered, or a PIN that does not match, exits
with status 1.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := format.Parse(cmd.Flag("output").Value.String())
			if err != nil {
				return errs.WithCode(errs.ExitUsage, err)
			}
			if cmd.Flags().Changed("pin") && ro.pin == 0 {
				return errs.WithCode(errs.ExitUsage, errors.New("--pin must not be 0"))
			}
			res, err := register(cmd.Context(), opts, ro)
			if f == format.JSON {
				if werr := format.WriteJSONResult(cmd.OutOrStdout(), res, err); werr != nil {
					return werr
				}
				return err
			}
			if res != nil {
				if werr := printRegister(cmd.OutOrStdout(), res); werr != nil {
					return werr
				}
			}
			return err
		},
	}
	cmd.Flags().Uint32Var(&ro.pin, "pin", 0, "PIN the device was given out of band, to check against its Registration")
	cmd.Flags().BoolVar(&ro.noCreate, "no-create", false, "only look the EndDevice up; do not create it")
	format.AddFlag(cmd)
	return cmd
}

// register runs the registration. It returns what it found even when it
// fails after finding the EndDevice.
func register(ctx context.Context, opts *options, ro *registerOptions) (*RegisterResult, error) {
	c, id, err := opts.connect()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	dc, err := c.DeviceCapability(ctx)
	if err != nil {
		return nil, classify(err)
	}
	if dc.EndDeviceListLink == nil || dc.EndDeviceListLink.Href == "" {
		return nil, errs.WithCode(errs.ExitError, errors.New("the server's DeviceCapability links no EndDeviceList"))
	}
	res := &RegisterResult{LFDI: id.LFDI.String(), SFDI: uint64(id.SFDI)}
	list := dc.EndDeviceListLink.Href

	var ed *ieee20305.EndDevice
	if ro.noCreate {
		if ed, err = c.FindEndDevice(ctx, list, id); err == nil && ed == nil {
			return res, errs.WithCode(errs.ExitError, fmt.Errorf("the server has no EndDevice with SFDI %d", id.SFDI))
		}
	} else {
		ed, res.Created, err = c.Register(ctx, list, id)
	}
	if err != nil {
		return res, classify(err)
	}
	if ed.Href != nil {
		res.EndDevice = *ed.Href
	}

	reg, err := c.Registration(ctx, ed)
	switch {
	case errors.Is(err, ieeeclient.ErrNotRegistered) && ro.pin != 0:
		return res, errs.WithCode(errs.ExitError, fmt.Errorf("--pin cannot be checked: %w", err))
	case errors.Is(err, ieeeclient.ErrNotRegistered):
		return res, nil
	case err != nil:
		return res, classify(err)
	}
	res.Registration = &RegistrationInfo{
		Href:       ed.RegistrationLink.Href,
		PIN:        uint32(reg.PIN),
		Registered: time.Unix(int64(reg.DateTimeRegistered), 0).UTC(),
		PollRate:   uint32(reg.GetPollRate()),
	}
	if ro.pin != 0 {
		if err := ieeeclient.CheckPIN(reg, ro.pin); err != nil {
			return res, errs.WithCode(errs.ExitError, err)
		}
		res.Registration.PINVerified = true
	}
	return res, nil
}

func printRegister(w io.Writer, r *RegisterResult) error {
	state := "found"
	if r.Created {
		state = "created"
	}
	var b []byte
	b = fmt.Appendf(b, "LFDI:          %s\nSFDI:          %d\n", r.LFDI, r.SFDI)
	if r.EndDevice != "" {
		b = fmt.Appendf(b, "EndDevice:     %s (%s)\n", r.EndDevice, state)
	}
	switch reg := r.Registration; {
	case reg == nil && r.EndDevice != "":
		b = fmt.Appendf(b, "Registration:  none: the server has not registered the device\n")
	case reg != nil:
		check := "not checked (no --pin)"
		if reg.PINVerified {
			check = "matches --pin"
		}
		b = fmt.Appendf(b, "Registration:  %s\n  PIN:         %d (%s)\n  Registered:  %s\n  Poll rate:   %d s\n",
			reg.Href, reg.PIN, check, reg.Registered.Format(time.RFC3339), reg.PollRate)
	}
	_, err := w.Write(b)
	return err
}

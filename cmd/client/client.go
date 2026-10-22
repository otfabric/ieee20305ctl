// SPDX-License-Identifier: MIT

// Package client is the device side of ieee20305ctl: the commands that talk
// to an IEEE 2030.5 server over HTTPS with mutual TLS, as a device.
package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/go-ieee20305/schedule"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/runner"
)

// NewCmd returns the client command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Talk to an IEEE 2030.5 server as a device",
		Long: `Commands of an IEEE 2030.5 device. Each connects to the server at --url over HTTPS
with mutual TLS: the device authenticates with its certificate (--cert, --key), and
verifies the server with --ca. Servers that offer only the cipher suite
TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 (GridAPPS-D, the EPRI stack) need --ccm8.

The flags of the connection can come from the environment: IEEE20305CTL_URL,
IEEE20305CTL_CERT, IEEE20305CTL_KEY, IEEE20305CTL_CA, and so on.`,
	}
	cmd.AddCommand(
		newDiscoverCmd(), newDcapCmd(), newTimeCmd(), newRegisterCmd(), newGetCmd(), newWalkCmd(),
		newProgramsCmd(), newControlsCmd(), newRespondCmd(), newDERCmd(),
		newReportCmd(), newLogEventCmd(), newPricingCmd(), newFlowReservationCmd(),
		newPutCmd(), newPostCmd(), newDeleteCmd(),
		newSubscribeCmd(), newRunCmd(), newConnectionPointCmd(),
	)
	return cmd
}

// Common are the flags of every client command.
type Common struct {
	conn.Config
	Format string `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newCommon() Common { return Common{Config: conn.Defaults(), Format: "text"} }

// once is a client command with one result: it validates the flags,
// connects, runs, and writes what run returns. A failure of run is
// classified (conn.Classify); write says whether run may change the server.
func once(cmd *cobra.Command, c *Common, validate func() error, write bool, run func(ctx context.Context, d *conn.Device) (any, error)) error {
	return runner.RunFormattedChecked(cmd, c.Format, func() error {
		if err := c.Validate(); err != nil {
			return err
		}
		if validate != nil {
			return validate()
		}
		return nil
	}, func(ctx context.Context) (any, error) {
		d, err := c.Connect()
		if err != nil {
			return nil, err
		}
		v, err := run(ctx, d)
		if err != nil {
			return nil, conn.Classify(err, write)
		}
		return v, nil
	})
}

// endDevice returns the DeviceCapability and the EndDevice of the device,
// which the server must have.
func endDevice(ctx context.Context, d *conn.Device) (*ieee20305.DeviceCapability, *ieee20305.EndDevice, error) {
	dcap, err := d.Client.DeviceCapability(ctx)
	if err != nil {
		return nil, nil, err
	}
	if dcap.EndDeviceListLink == nil {
		return nil, nil, errs.New(errs.KindProtocol, errs.CodeInvalidResponse, "the server's DeviceCapability has no EndDeviceListLink", nil)
	}
	ed, err := d.Client.FindEndDevice(ctx, dcap.EndDeviceListLink.Href, d.Identity)
	if err != nil {
		return nil, nil, err
	}
	if ed == nil {
		return nil, nil, errs.New(errs.KindProtocol, errs.CodeNotRegistered,
			fmt.Sprintf("the server has no EndDevice for SFDI %d; run client register first", d.Identity.SFDI), nil)
	}
	return dcap, ed, nil
}

// fsas returns the FunctionSetAssignments of ed.
func fsas(ctx context.Context, d *conn.Device, ed *ieee20305.EndDevice) ([]*ieee20305.FunctionSetAssignments, error) {
	var out []*ieee20305.FunctionSetAssignments
	if ed.FunctionSetAssignmentsListLink == nil {
		return nil, nil
	}
	err := d.Client.Iterate(ctx, ed.FunctionSetAssignmentsListLink.Href, 50, func(it any) error {
		if f, ok := it.(*ieee20305.FunctionSetAssignments); ok {
			out = append(out, f)
		}
		return nil
	})
	return out, err
}

func hexOf(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }

func str(s *ieee20305.String32) string {
	if s == nil {
		return ""
	}
	return string(*s)
}

func statusText(s *ieee20305.EventStatus) string {
	if s == nil {
		return "unknown"
	}
	return schedule.Status(s.CurrentStatus).String()
}

// pretty indents an XML document.
func pretty(data []byte) string {
	var out bytes.Buffer
	dec := xml.NewDecoder(bytes.NewReader(data))
	enc := xml.NewEncoder(&out)
	enc.Indent("", "  ")
	for {
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return string(data)
		}
		if cd, ok := tok.(xml.CharData); ok && len(bytes.TrimSpace(cd)) == 0 {
			continue
		}
		if err := enc.EncodeToken(tok); err != nil {
			return string(data)
		}
	}
	if err := enc.Flush(); err != nil {
		return string(data)
	}
	return out.String()
}

// marshal encodes v in the device's codec, indented.
func marshal(d *conn.Device, v any) (string, error) {
	data, err := d.Codec.Marshal(v)
	if err != nil {
		return "", err
	}
	return pretty(data), nil
}

// elementName is the document element of v.
func elementName(v any) string {
	if n, ok := csipaus.ElementName(v); ok {
		return n.Local
	}
	if n, ok := ieee20305.ElementName(v); ok {
		return n.Local
	}
	return fmt.Sprintf("%T", v)
}

func unix(t ieee20305.TimeType) time.Time { return time.Unix(int64(t), 0).UTC() }

// confirmed refuses a command that changes the server without --yes.
func confirmed(yes bool, what string) error {
	if !yes {
		return errs.New(errs.KindUsage, errs.CodeConfirmationRequired, what+" changes the server: confirm with --yes", nil)
	}
	return nil
}

var _ = ieeeclient.IdentityOf

// listOne asks for the first item of a list.
var listOne = ieeeclient.ListOptions{Limit: 1}

// quietLog discards the errors of an http.Server: handshakes that fail are
// the peer's business.
func quietLog() *log.Logger { return log.New(io.Discard, "", 0) }

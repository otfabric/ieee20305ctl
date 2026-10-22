// SPDX-License-Identifier: MIT

// Package server is the server side of ieee20305ctl: an IEEE 2030.5 server
// seeded from a file, and the operator commands that act on it while it
// runs, through its control socket.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/go-ieee20305/discovery"
	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/go-ieee20305/tlsccm"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/ieee"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/site"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

// NewCmd returns the server command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Serve IEEE 2030.5 to devices, and operate the server while it runs",
		Long: `"server start" serves IEEE 2030.5 over HTTPS with mutual TLS, from a seed file: the
devices it knows with their PINs, and the DER programs, controls and tariffs it assigns
them (server generate-config writes an example). It writes a record per thing that
happens: devices registering, requests (with --log-requests), Responses, notifications,
event status changes.

With --control-socket, an operator acts on the running server: issue-control and cancel
change the controls devices receive, set-default their default control, provision and
register add devices, devices and get read what the server holds.`,
	}
	cmd.AddCommand(newStartCmd(), newGenerateConfigCmd(), newDevicesCmd(), newGetCmd(),
		newIssueControlCmd(), newCancelCmd(), newSetDefaultCmd(), newProvisionCmd(), newRegisterCmd())
	return cmd
}

type startConfig struct {
	Listen         string        `flag:"listen" env:"IEEE20305CTL_LISTEN" desc:"Address to listen on"`
	Cert           string        `flag:"cert" env:"IEEE20305CTL_SERVER_CERT" desc:"Server certificate (PEM)"`
	Key            string        `flag:"key" env:"IEEE20305CTL_SERVER_KEY" desc:"Private key of the server certificate (PEM)"`
	CA             string        `flag:"ca" env:"IEEE20305CTL_CA" desc:"CA certificates of the devices (PEM)"`
	Config         string        `flag:"config" env:"IEEE20305CTL_SERVER_CONFIG" desc:"Seed file (YAML or JSON): devices, programs, tariffs; see server generate-config"`
	CCM8           bool          `flag:"ccm8" desc:"Serve TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first, then AES-128-GCM (for GridAPPS-D and EPRI clients)"`
	Profile        string        `flag:"profile" env:"IEEE20305CTL_PROFILE" desc:"Profile: ieee20305 or csipaus (CSIP-AUS documents, connection points, IEEE 2030.5-2018)"`
	Edition        string        `flag:"edition" env:"IEEE20305CTL_EDITION" desc:"Edition of devices the seed does not give one: 2023 or 2018"`
	InBand         bool          `flag:"in-band" desc:"Let unknown devices post their EndDevice (in-band registration)"`
	ControlSocket  string        `flag:"control-socket" env:"IEEE20305CTL_CONTROL_SOCKET" desc:"Unix socket of the operator commands (none when empty)"`
	Duration       time.Duration `flag:"duration" desc:"Stop after this long (0: until interrupted)"`
	LogRequests    bool          `flag:"log-requests" desc:"Write a record for every request"`
	NotifyLoopback bool          `flag:"notify-loopback" desc:"Deliver notifications to loopback and link-local addresses too (for local tests)"`
	Advertise      bool          `flag:"advertise" desc:"Advertise the server with DNS-SD (_smartenergy._tcp) on the local network"`
	Format         string        `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newStartCmd() *cobra.Command {
	c := startConfig{Listen: "0.0.0.0:8443", Profile: "ieee20305", Edition: "2023", Format: "text"}
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Serve IEEE 2030.5 to devices",
		Long: `Serve IEEE 2030.5 over HTTPS with mutual TLS on --listen, seeded from --config, until
interrupted or --duration. The server presents --cert, and accepts devices whose
certificates --ca verifies. Its DeviceCapability is at /dcap.

Records: listening; device_provisioned per device of the seed; control_issued per
control; then log (registrations, subscriptions), response, notification, event_status
and, with --log-requests, request. With --format json the records are JSON Lines.`,
		Example: `  ieee20305ctl server start --cert server.pem --key server.key --ca ca.pem --config seed.yaml
  ieee20305ctl server start --cert server.pem --key server.key --ca ca.pem --config seed.yaml \
      --control-socket /tmp/ieee20305.sock --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runner.RunStream(cmd, c.Format, c.validate, c.run)
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func (c *startConfig) validate() error {
	if c.Cert == "" || c.Key == "" || c.CA == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--cert, --key and --ca are required: IEEE 2030.5 is mutual TLS", nil)
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--listen %q: host:port", c.Listen), err)
	}
	switch c.Profile {
	case "ieee20305", "csipaus":
	default:
		return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--profile %q: ieee20305 or csipaus", c.Profile), nil)
	}
	if _, err := ieee20305.ParseEdition(c.Edition); err != nil {
		return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--edition %q: 2023 or 2018", c.Edition), err)
	}
	return nil
}

func (c *startConfig) run(ctx context.Context, out *format.StreamWriter) (int, error) {
	cert, err := conn.LoadKeyPair(c.Cert, c.Key)
	if err != nil {
		return 0, err
	}
	roots, err := conn.LoadPool(c.CA)
	if err != nil {
		return 0, err
	}
	var seed *site.Seed
	if c.Config != "" {
		if seed, err = site.LoadSeed(c.Config); err != nil {
			return 0, errs.InvalidInput(errs.CodeInvalidConfig, err.Error(), err)
		}
	}
	if c.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Duration)
		defer cancel()
	}
	var mu sync.Mutex
	var werr error
	emit := func(e *types.Event) {
		mu.Lock()
		defer mu.Unlock()
		if werr == nil {
			werr = out.Write(e)
		}
	}
	edition, _ := ieee20305.ParseEdition(c.Edition)
	inner, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return 0, errs.New(errs.KindTransport, errs.CodeListenFailed, fmt.Sprintf("listen on %s: %v", c.Listen, err), err)
	}
	addr := inner.Addr().String()
	emit(&types.Event{Time: time.Now().UTC(), Kind: "listening", Address: addr,
		Message: fmt.Sprintf("https://%s/dcap (%s, %s)", addr, c.Profile, edition)})
	s, err := site.New(ctx, site.Options{Profile: c.Profile, Edition: edition, InBand: c.InBand, ServerCert: cert,
		DeviceCAs: roots, CCM8: c.CCM8, NotifyLoopback: c.NotifyLoopback, LogRequests: c.LogRequests, Events: emit}, seed)
	if err != nil {
		_ = inner.Close()
		var e *errs.Error
		if errors.As(err, &e) {
			return 0, err
		}
		return 0, errs.InvalidInput(errs.CodeInvalidConfig, "the seed: "+err.Error(), err)
	}
	defer func() { _ = s.Close() }()
	if c.ControlSocket != "" {
		if err := s.ServeControl(ctx, c.ControlSocket); err != nil {
			_ = inner.Close()
			return 0, errs.New(errs.KindTransport, errs.CodeControlSocketFailed, fmt.Sprintf("the control socket %s: %v", c.ControlSocket, err), err)
		}
		emit(&types.Event{Time: time.Now().UTC(), Kind: "control_socket", Address: c.ControlSocket})
	}
	l := net.Listener(inner)
	hs := &http.Server{Handler: s.Server, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	if c.CCM8 {
		if l, err = tlsccm.NewListener(inner, cert, roots, tlsccm.Options{AllowGCM: true}); err != nil {
			_ = inner.Close()
			return 0, errs.InvalidInput(errs.CodeInvalidCertificate, err.Error(), err)
		}
		tlsccm.ConfigureServer(hs)
	} else {
		l = tls.NewListener(inner, security.ServerTLSConfig(cert, roots))
	}
	go func() { _ = hs.Serve(l) }()
	defer func() { _ = hs.Close() }()
	if c.Advertise {
		port, _ := strconv.Atoi(addr[strings.LastIndex(addr, ":")+1:])
		host, _ := os.Hostname()
		adv, err := discovery.Advertise(ctx, discovery.Service{Instance: "ieee20305ctl " + host, Port: port,
			TXT: map[string]string{discovery.KeyTXTVersion: "1", discovery.KeyDcap: "/dcap", discovery.KeyHTTPS: strconv.Itoa(port)}},
			discovery.AdvertiseOptions{})
		if err != nil {
			emit(&types.Event{Time: time.Now().UTC(), Kind: "warning", Message: "DNS-SD advertisement: " + err.Error()})
		} else {
			defer func() { _ = adv.Close() }()
			emit(&types.Event{Time: time.Now().UTC(), Kind: "advertised", Message: discovery.ServiceType + " port " + strconv.Itoa(port)})
		}
	}
	<-ctx.Done()
	mu.Lock()
	defer mu.Unlock()
	if werr != nil {
		return 0, errs.Output(errs.CodeOutputWriteFailed, werr)
	}
	return errs.ExitOK, nil
}

type generateConfig struct {
	Output string `flag:"output" desc:"Write to this file instead of stdout"`
	Force  bool   `flag:"force" desc:"Overwrite --output"`
}

func newGenerateConfigCmd() *cobra.Command {
	c := generateConfig{}
	cmd := &cobra.Command{
		Use:     "generate-config",
		Short:   "Write an example seed file, commented",
		Example: `  ieee20305ctl server generate-config --output seed.yaml`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.Output == "" {
				_, err := io.WriteString(cmd.OutOrStdout(), site.Example)
				return err
			}
			if _, err := os.Stat(c.Output); err == nil && !c.Force {
				return errs.InvalidInput(errs.CodeOutputFileCreateFailed, c.Output+" exists; use --force to overwrite", nil)
			}
			if err := os.WriteFile(c.Output, []byte(site.Example), 0o644); err != nil {
				return errs.Output(errs.CodeOutputFileWriteFailed, err)
			}
			_, err := fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", c.Output)
			return err
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// ControlConfig are the flags of the operator commands.
type ControlConfig struct {
	Socket  string        `flag:"control-socket" env:"IEEE20305CTL_CONTROL_SOCKET" desc:"Unix socket of the running server start"`
	Timeout time.Duration `flag:"timeout" desc:"Time limit of the operation"`
	Format  string        `flag:"format" env:"IEEE20305CTL_FORMAT" desc:"Output format: text, json or table"`
}

func newControl() ControlConfig { return ControlConfig{Timeout: 30 * time.Second, Format: "text"} }

func (c *ControlConfig) validate() error {
	if c.Socket == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--control-socket is required (or IEEE20305CTL_CONTROL_SOCKET)", nil)
	}
	return nil
}

// call runs one operation on the control socket and writes its result.
func call(cmd *cobra.Command, c *ControlConfig, validate func() error, req func() (site.Request, any, error)) error {
	return runner.RunFormattedChecked(cmd, c.Format, func() error {
		if err := c.validate(); err != nil {
			return err
		}
		if validate != nil {
			return validate()
		}
		return nil
	}, func(ctx context.Context) (any, error) {
		r, result, err := req()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		if err := site.Call(ctx, c.Socket, r, result); err != nil {
			return nil, err
		}
		return result, nil
	})
}

// Devices is the result of server devices.
type Devices struct {
	Devices []site.DeviceInfo `json:"devices"`
}

func (d *Devices) MarshalTextOutput() (string, error) {
	if len(d.Devices) == 0 {
		return "no devices", nil
	}
	var b strings.Builder
	for _, x := range d.Devices {
		state := "not registered"
		if x.Registered {
			state = fmt.Sprintf("registered, PIN %d", x.PIN)
		}
		fmt.Fprintf(&b, "%s  LFDI %s  SFDI %s  %s\n", x.EndDevice, x.LFDI, x.SFDI, state)
	}
	return b.String(), nil
}

func (d *Devices) TableHeaders() []string {
	return []string{"END_DEVICE", "LFDI", "SFDI", "REGISTERED", "PIN"}
}
func (d *Devices) TableRows() [][]string {
	rows := [][]string{}
	for _, x := range d.Devices {
		rows = append(rows, []string{x.EndDevice, x.LFDI, x.SFDI, strconv.FormatBool(x.Registered), strconv.FormatUint(uint64(x.PIN), 10)})
	}
	return rows
}

func newDevicesCmd() *cobra.Command {
	c := newControl()
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "List the devices of the running server",
		Args:  cobra.NoArgs,
	}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var list []site.DeviceInfo
		return runner.RunFormattedChecked(cmd, c.Format, c.validate, func(ctx context.Context) (any, error) {
			ctx, cancel := context.WithTimeout(ctx, c.Timeout)
			defer cancel()
			if err := site.Call(ctx, c.Socket, site.Request{Op: "devices"}, &list); err != nil {
				return nil, err
			}
			if list == nil {
				list = []site.DeviceInfo{}
			}
			return &Devices{Devices: list}, nil
		})
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// Element is the result of server get.
type Element struct{ site.Element }

func (e *Element) MarshalTextOutput() (string, error) { return e.XML, nil }

func newGetCmd() *cobra.Command {
	c := newControl()
	cmd := &cobra.Command{
		Use:     "get <href>",
		Short:   "Read a resource of the running server, as the server holds it",
		Example: `  ieee20305ctl server get /edev/1/rsps --control-socket /tmp/ieee20305.sock`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return call(cmd, &c, nil, func() (site.Request, any, error) {
				return site.Request{Op: "get", Href: args[0]}, &Element{}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func confirmed(yes bool, what string) error {
	if !yes {
		return errs.New(errs.KindUsage, errs.CodeConfirmationRequired, what+" changes what devices receive: confirm with --yes", nil)
	}
	return nil
}

type issueConfig struct {
	ControlConfig
	Program     string        `flag:"program" desc:"Program: its href or description (default: the only one)"`
	Description string        `flag:"description" desc:"Description of the control"`
	Set         []string      `flag:"set" desc:"Controls as name=value: opModConnect=false, opModMaxLimW=50 (percent), opModExpLimW=1500 (W), ..."`
	Start       time.Duration `flag:"start" desc:"Start, from now"`
	Duration    time.Duration `flag:"duration" desc:"Length of the control"`
	NoResponses bool          `flag:"no-responses" desc:"Ask for no Responses"`
	Yes         bool          `flag:"yes" desc:"Confirm: devices act on the control"`
}

func newIssueControlCmd() *cobra.Command {
	c := issueConfig{ControlConfig: newControl(), Duration: time.Hour}
	cmd := &cobra.Command{
		Use:   "issue-control",
		Short: "Issue a DER control to a program of the running server",
		Long: `Create a DER control in a program: the devices of the program receive it at their next
poll (or notification), schedule it, and answer it with Responses. Settings by schema name:
` + strings.Join(ieee.Names(), ", ") + `.`,
		Example: `  ieee20305ctl server issue-control --program "Utility program" --set opModMaxLimW=30 --duration 30m --yes
  ieee20305ctl server issue-control --set opModExpLimW=1500 --start 5m --duration 1h --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return call(cmd, &c.ControlConfig, func() error {
				if err := confirmed(c.Yes, "issue-control"); err != nil {
					return err
				}
				if len(c.Set) == 0 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--set at least one control", nil)
				}
				if _, err := ieee.ParseSettings(c.Set); err != nil {
					return errs.InvalidInput(errs.CodeInvalidInput, err.Error(), err)
				}
				if c.Duration <= 0 || c.Start < 0 {
					return errs.InvalidInput(errs.CodeInvalidInput, "--duration must be positive, and --start not negative", nil)
				}
				return nil
			}, func() (site.Request, any, error) {
				ctl := &site.ControlRequest{Program: c.Program, Description: c.Description, Set: c.Set,
					StartAt: time.Now().Add(c.Start), Duration: c.Duration, NoResponses: c.NoResponses}
				return site.Request{Op: "issue-control", Control: ctl}, &site.Created{Action: "issued"}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type cancelConfig struct {
	ControlConfig
	Yes bool `flag:"yes" desc:"Confirm: devices stop the control"`
}

func newCancelCmd() *cobra.Command {
	c := cancelConfig{ControlConfig: newControl()}
	cmd := &cobra.Command{
		Use:     "cancel <control-href>",
		Short:   "Cancel a DER control of the running server",
		Example: `  ieee20305ctl server cancel /derp/1/derc/2 --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return call(cmd, &c.ControlConfig, func() error { return confirmed(c.Yes, "cancel") }, func() (site.Request, any, error) {
				return site.Request{Op: "cancel", Href: args[0]}, &site.Created{Action: "cancelled"}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type setDefaultConfig struct {
	ControlConfig
	Program string   `flag:"program" desc:"Program: its href or description (default: the only one)"`
	Set     []string `flag:"set" desc:"Controls as name=value, as issue-control takes them"`
	Yes     bool     `flag:"yes" desc:"Confirm: devices apply it when no control does"`
}

func newSetDefaultCmd() *cobra.Command {
	c := setDefaultConfig{ControlConfig: newControl()}
	cmd := &cobra.Command{
		Use:     "set-default",
		Short:   "Set the default control of a program of the running server",
		Example: `  ieee20305ctl server set-default --program "Utility program" --set opModConnect=true --set opModMaxLimW=80 --yes`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return call(cmd, &c.ControlConfig, func() error {
				if err := confirmed(c.Yes, "set-default"); err != nil {
					return err
				}
				if _, err := ieee.ParseSettings(c.Set); err != nil {
					return errs.InvalidInput(errs.CodeInvalidInput, err.Error(), err)
				}
				return nil
			}, func() (site.Request, any, error) {
				return site.Request{Op: "set-default", Program: c.Program, Set: c.Set}, &site.Created{Action: "default set"}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type provisionConfig struct {
	ControlConfig
	Cert         string `flag:"cert" desc:"Certificate of the device (PEM)"`
	LFDI         string `flag:"lfdi" desc:"LFDI of the device, instead of --cert"`
	PIN          uint32 `flag:"pin" desc:"Registration PIN"`
	Edition      string `flag:"edition" desc:"Edition of the device: 2023 or 2018 (default: the server's)"`
	Role         string `flag:"role" desc:"device, aggregator or operator"`
	Unregistered bool   `flag:"unregistered" desc:"Leave the device unregistered (server register later)"`
}

func newProvisionCmd() *cobra.Command {
	c := provisionConfig{ControlConfig: newControl(), Role: "device"}
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Tell the running server of a device",
		Long: `Provision a device on the running server: its EndDevice, its Registration with --pin, and
for a device the programs and tariffs of the seed.`,
		Example: `  ieee20305ctl server provision --cert device-2.pem --pin 222226`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return call(cmd, &c.ControlConfig, func() error {
				if (c.Cert == "") == (c.LFDI == "") {
					return errs.InvalidInput(errs.CodeInvalidFlagCombination, "give --cert or --lfdi", nil)
				}
				return nil
			}, func() (site.Request, any, error) {
				d := &site.DeviceSeed{LFDI: c.LFDI, PIN: c.PIN, Edition: c.Edition, Role: c.Role, Unregistered: c.Unregistered}
				if c.Cert != "" {
					cert, err := conn.LoadCertificate(c.Cert)
					if err != nil {
						return site.Request{}, nil, err
					}
					d.LFDI = security.CertificateLFDI(cert).String()
				}
				return site.Request{Op: "provision", Device: d}, &site.Created{Action: "provisioned"}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type registerConfig struct {
	ControlConfig
	LFDI string `flag:"lfdi" desc:"LFDI of the device"`
	PIN  uint32 `flag:"pin" desc:"Registration PIN"`
}

func newRegisterCmd() *cobra.Command {
	c := registerConfig{ControlConfig: newControl()}
	cmd := &cobra.Command{
		Use:     "register",
		Short:   "Register a device the running server knows (in-band or unregistered)",
		Example: `  ieee20305ctl server register --lfdi 3E4F45AB31EDFE5B67E343E5E4562E31984E23E5 --pin 111115`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return call(cmd, &c.ControlConfig, func() error {
				if _, err := security.ParseLFDI(c.LFDI); err != nil {
					return errs.InvalidInput(errs.CodeInvalidInput, "--lfdi: "+err.Error(), err)
				}
				return nil
			}, func() (site.Request, any, error) {
				return site.Request{Op: "register", LFDI: c.LFDI, PIN: c.PIN}, &site.Created{Action: "registered"}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/go-ieee20305/csipaus/csipclient"
	"github.com/otfabric/go-ieee20305/security"
	"github.com/spf13/cobra"

	"github.com/otfabric/ieee20305ctl/internal/conn"
	"github.com/otfabric/ieee20305ctl/internal/errs"
	"github.com/otfabric/ieee20305ctl/internal/format"
	"github.com/otfabric/ieee20305ctl/internal/runner"
	"github.com/otfabric/ieee20305ctl/internal/types"
)

type fileConfig struct {
	Common
	File string `flag:"file" desc:"File of the document (XML), checked against the schema before it is sent"`
}

func newPutCmd() *cobra.Command {
	c := fileConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "put <href>",
		Short: "Replace a resource of the device with a document from a file",
		Long: `PUT the XML document of --file to href: DeviceInformation, DeviceStatus, PowerStatus,
DERSettings and the other resources a device keeps up to date.`,
		Example: `  ieee20305ctl client put /edev/1/di --file deviceinformation.xml`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return once(cmd, &c.Common, c.validate, true, func(ctx context.Context, d *conn.Device) (any, error) {
				doc, err := readDocument(d, c.File)
				if err != nil {
					return nil, err
				}
				if err := d.Client.Put(ctx, args[0], doc); err != nil {
					return nil, err
				}
				return &types.Written{Action: "put", Hrefs: []string{args[0]}}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func (c *fileConfig) validate() error {
	if c.File == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--file is required", nil)
	}
	return nil
}

func newPostCmd() *cobra.Command {
	c := fileConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:     "post <list-href>",
		Short:   "Create a resource in a list from a document in a file",
		Long:    `POST the XML document of --file to the list at list-href, and print the Location of what the server created.`,
		Example: `  ieee20305ctl client post /edev/1/lel --file logevent.xml`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return once(cmd, &c.Common, c.validate, true, func(ctx context.Context, d *conn.Device) (any, error) {
				doc, err := readDocument(d, c.File)
				if err != nil {
					return nil, err
				}
				loc, err := d.Client.Post(ctx, args[0], doc)
				if err != nil {
					return nil, err
				}
				return &types.Posted{Action: "post", Target: args[0], Location: loc, Detail: elementName(doc)}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type deleteConfig struct {
	Common
	Yes bool `flag:"yes" desc:"Confirm the deletion"`
}

func newDeleteCmd() *cobra.Command {
	c := deleteConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:     "delete <href>",
		Short:   "Delete a resource of the device (a subscription, say)",
		Example: `  ieee20305ctl client delete /edev/1/sub/2 --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return once(cmd, &c.Common, func() error { return confirmed(c.Yes, "delete") }, true, func(ctx context.Context, d *conn.Device) (any, error) {
				if err := d.Client.Delete(ctx, args[0]); err != nil {
					return nil, err
				}
				return &types.Written{Action: "deleted", Hrefs: []string{args[0]}}, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type connectionPointConfig struct {
	Common
	Set string `flag:"set" desc:"Put this connection point ID (the NMI in Australia) before reading it"`
}

func newConnectionPointCmd() *cobra.Command {
	c := connectionPointConfig{Common: newCommon()}
	cmd := &cobra.Command{
		Use:   "connection-point",
		Short: "Read or set the CSIP-AUS connection point of the device",
		Long: `Read the ConnectionPoint that CSIP-AUS adds to the device's EndDevice, or set its ID with
--set: the National Metering Identifier of the site in Australia. Implies --profile csipaus.`,
		Example: `  ieee20305ctl client connection-point --set 4407123456`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c.Profile = "csipaus"
			return once(cmd, &c.Common, nil, c.Set != "", func(ctx context.Context, d *conn.Device) (any, error) {
				_, base, err := endDevice(ctx, d)
				if err != nil {
					return nil, err
				}
				ed, err := csipclient.EndDevice(ctx, d.Client, base.GetHref())
				if err != nil {
					return nil, err
				}
				if ed.ConnectionPointLink == nil {
					return nil, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the EndDevice has no CSIP-AUS ConnectionPointLink: the server is not of CSIP-AUS", nil)
				}
				res := &types.ConnectionPoint{EndDevice: ed.GetHref(), Href: ed.ConnectionPointLink.Href}
				if c.Set != "" {
					if err := csipclient.SetConnectionPoint(ctx, d.Client, ed, c.Set); err != nil {
						return nil, err
					}
					res.Set = true
				}
				cp, err := csipclient.ConnectionPoint(ctx, d.Client, ed)
				if errors.Is(err, ieee20305.ErrNotFound) {
					return res, nil
				}
				if err != nil {
					return nil, err
				}
				res.ID = string(cp.ConnectionPointId)
				return res, nil
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

type subscribeConfig struct {
	Common
	Listen      string        `flag:"listen" desc:"Address the notification listener binds"`
	NotifyURL   string        `flag:"notify-url" desc:"URL the server posts notifications to (default: https://<--listen>/notify, which needs a --listen address the server can reach)"`
	ListenCert  string        `flag:"listen-cert" desc:"Certificate of the listener (default: the device certificate)"`
	ListenKey   string        `flag:"listen-key" desc:"Key of the listener (default: the device key)"`
	Limit       uint32        `flag:"limit" desc:"Most items of a list a notification carries"`
	Count       int           `flag:"count" desc:"Stop after this many notifications (0: until interrupted or --duration)"`
	Duration    time.Duration `flag:"duration" desc:"Stop after this long (0: until interrupted or --count)"`
	KeepOnClose bool          `flag:"keep" desc:"Leave the subscription on the server when the command ends"`
}

func newSubscribeCmd() *cobra.Command {
	c := subscribeConfig{Common: newCommon(), Listen: "0.0.0.0:8443", Limit: 10}
	cmd := &cobra.Command{
		Use:   "subscribe <resource-href>",
		Short: "Subscribe to a resource and stream the notifications the server sends",
		Long: `Serve an HTTPS notification listener, subscribe to the resource at resource-href (a list,
such as the DERControlList of a program, or a resource), and write one record per event:
listening, subscribed, then one notification per change the server reports. The listener
verifies the server's certificate when it presents one. The subscription is deleted when
the command ends, unless --keep. With --format json the records are JSON Lines.`,
		Example: `  ieee20305ctl client subscribe /derp/1/derc --listen 0.0.0.0:8443 --notify-url https://10.0.0.7:8443/notify
  ieee20305ctl client subscribe /edev/1/fsa --count 1 --duration 5m --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.RunStream(cmd, c.Format, c.validate, func(ctx context.Context, out *format.StreamWriter) (int, error) {
				return c.run(ctx, out, args[0])
			})
		},
	}
	runner.RegisterCommandCfg(cmd, &c)
	return cmd
}

func (c *subscribeConfig) validate() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--listen %q: host:port", c.Listen), err)
	}
	if c.NotifyURL != "" {
		if u, err := url.Parse(c.NotifyURL); err != nil || u.Scheme != "https" {
			return errs.InvalidInput(errs.CodeInvalidURL, fmt.Sprintf("--notify-url %q: an https URL", c.NotifyURL), err)
		}
	}
	if (c.ListenCert == "") != (c.ListenKey == "") {
		return errs.InvalidInput(errs.CodeInvalidFlagCombination, "give --listen-cert and --listen-key together", nil)
	}
	return nil
}

// listen serves h on --listen over TLS, and returns the listener's URL.
func listen(ctx context.Context, addr string, cert tls.Certificate, d *conn.Device, h http.Handler) (string, func(), error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, errs.New(errs.KindTransport, errs.CodeListenFailed, fmt.Sprintf("listen on %s: %v", addr, err), err)
	}
	// A server's notifier presents its server certificate, whose key usage
	// is not clientAuth: verify it as IEEE 2030.5 does, with any usage.
	verify := security.VerifyPeerCertificate(security.VerifyOptions{Roots: d.Roots, Usage: x509.ExtKeyUsageAny})
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequestClientCert,
		VerifyPeerCertificate: func(raw [][]byte, chains [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return nil // some servers, such as GridAPPS-D, present none
			}
			return verify(raw, chains)
		}}
	hs := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, TLSConfig: cfg, ErrorLog: quietLog()}
	go func() { _ = hs.ServeTLS(l, "", "") }()
	return "https://" + l.Addr().String() + "/notify", func() { _ = hs.Close() }, nil
}

func (c *subscribeConfig) run(ctx context.Context, out *format.StreamWriter, resource string) (int, error) {
	d, err := c.Connect()
	if err != nil {
		return 0, err
	}
	_, ed, err := endDevice(ctx, d)
	if err != nil {
		return 0, conn.Classify(err, false)
	}
	if ed.SubscriptionListLink == nil {
		return 0, errs.New(errs.KindProtocol, errs.CodeUnknownResource, "the EndDevice has no SubscriptionListLink", nil)
	}
	cert := d.Cert
	if c.ListenCert != "" {
		if cert, err = conn.LoadKeyPair(c.ListenCert, c.ListenKey); err != nil {
			return 0, err
		}
	}
	if c.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Duration)
		defer cancel()
	}
	notes := make(chan *ieee20305.Notification, 16)
	h, err := ieeeclient.NewNotificationHandler(func(_ context.Context, n *ieee20305.Notification) error {
		select {
		case notes <- n:
		default:
		}
		return nil
	}, ieeeclient.NotificationConfig{Codec: d.Codec})
	if err != nil {
		return 0, errs.Internal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/notify", h)
	local, stop, err := listen(ctx, c.Listen, cert, d, mux)
	if err != nil {
		return 0, err
	}
	defer stop()
	uri := c.NotifyURL
	if uri == "" {
		uri = local
	}
	if err := out.Write(&types.Event{Time: now(), Kind: "listening", Address: local, Message: "notifications to " + uri}); err != nil {
		return 0, errs.Output(errs.CodeOutputWriteFailed, err)
	}
	sub, err := d.Client.Subscribe(ctx, ed.SubscriptionListLink.Href, resource, uri, c.Limit)
	if err != nil {
		return 0, conn.Classify(err, true)
	}
	if !c.KeepOnClose {
		defer func() {
			uctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if d.Client.Unsubscribe(uctx, sub) == nil {
				_ = out.Write(&types.Event{Time: now(), Kind: "unsubscribed", Href: sub})
			}
		}()
	}
	if err := out.Write(&types.Event{Time: now(), Kind: "subscribed", Href: sub, Resource: resource}); err != nil {
		return 0, errs.Output(errs.CodeOutputWriteFailed, err)
	}
	for n := 0; c.Count == 0 || n < c.Count; n++ {
		select {
		case <-ctx.Done():
			return errs.ExitOK, nil
		case note := <-notes:
			ev := &types.Event{Time: now(), Kind: "notification", Resource: note.SubscribedResource, Code: int(note.Status),
				Status: notificationStatus(note.Status)}
			if note.Resource != nil && note.Resource.Value != nil {
				ev.Message = elementName(note.Resource.Value)
				if r, ok := note.Resource.Value.(ieee20305.Hrefer); ok {
					ev.Href = r.GetHref()
				}
			}
			if err := out.Write(ev); err != nil {
				return 0, errs.Output(errs.CodeOutputWriteFailed, err)
			}
		}
	}
	return errs.ExitOK, nil
}

func notificationStatus(s ieee20305.UInt8) string {
	switch s {
	case 0:
		return "changed"
	case 1:
		return "cancelled-no-info"
	case 2:
		return "cancelled-moved"
	case 3:
		return "cancelled-deleted"
	case 4:
		return "cancelled-other"
	}
	return "status"
}

func now() time.Time { return time.Now().UTC() }

var _ = csipaus.Codec{}

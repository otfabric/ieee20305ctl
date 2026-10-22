// SPDX-License-Identifier: MIT

package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/ieee20305ctl/internal/cli"
	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// serverURL turns --host into the URL of the server.
func serverURL(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errs.WithCode(errs.ExitUsage, errors.New("--host is required"))
	}
	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return "", errs.WithCode(errs.ExitUsage, fmt.Errorf("--host %q: a URL must be https", host))
		}
		return strings.TrimSuffix(u.String(), "/"), nil
	}
	return "https://" + cli.ParseAddress(host, "443"), nil
}

// connect builds the IEEE 2030.5 client the flags describe.
func (o *options) connect() (*ieeeclient.Client, error) {
	server, err := serverURL(o.host)
	if err != nil {
		return nil, err
	}
	if o.certFile == "" || o.keyFile == "" {
		return nil, errs.WithCode(errs.ExitUsage, errors.New("--cert and --key are required: IEEE 2030.5 servers authenticate the device by its certificate"))
	}
	cert, err := cli.LoadTLSCertificate(o.certFile, o.keyFile)
	if err != nil {
		return nil, errs.WithCode(errs.ExitCertificate, err)
	}
	var roots *x509.CertPool
	if o.caFile != "" {
		if roots, err = cli.LoadCACertPool(o.caFile); err != nil {
			return nil, errs.WithCode(errs.ExitCertificate, err)
		}
	}
	c, err := ieeeclient.New(ieeeclient.Config{
		Server:               server,
		DeviceCapabilityPath: o.dcap,
		HTTPClient:           ieeeclient.NewHTTPClient(cert, roots),
		UserAgent:            "ieee20305ctl",
	})
	if err != nil {
		return nil, errs.WithCode(errs.ExitUsage, err)
	}
	return c, nil
}

// classify maps the failure of a call to a server to an exit code: no
// connection or no TLS session is ExitConnect; a refused certificate is
// ExitCertificate; an answer the server gave is ExitError.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var (
		status    *ieee20305.StatusError
		unknownCA x509.UnknownAuthorityError
		hostname  x509.HostnameError
		invalid   x509.CertificateInvalidError
		verify    *tls.CertificateVerificationError
		alert     tls.AlertError
		opErr     *net.OpError
		recordHdr tls.RecordHeaderError
		dnsErr    *net.DNSError
	)
	switch {
	case errors.As(err, &status), errors.Is(err, ieee20305.ErrProtocol):
		return errs.WithCode(errs.ExitError, err)
	case errors.As(err, &unknownCA), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verify):
		return errs.WithCode(errs.ExitCertificate, fmt.Errorf("the server's certificate was not accepted: %w", err))
	case errors.As(err, &alert), errors.As(err, &opErr) && opErr.Op == "remote error":
		// crypto/tls reports an alert the server sent as a *net.OpError with
		// Op "remote error"; during the handshake it is about the certificate.
		return errs.WithCode(errs.ExitCertificate, fmt.Errorf("the TLS handshake failed; the server may not accept the device certificate: %w", err))
	case errors.As(err, &recordHdr), errors.As(err, &opErr), errors.As(err, &dnsErr), errors.Is(err, ieee20305.ErrTimeout):
		return errs.WithCode(errs.ExitConnect, err)
	}
	return errs.WithCode(errs.ExitError, err)
}

// SPDX-License-Identifier: MIT

// Package conn connects ieee20305ctl to an IEEE 2030.5 server as a device:
// the flags every client command shares, the go-ieee20305 client they make,
// and the classification of what goes wrong into the error kinds of the
// tool (docs/exitcodes.md).
package conn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/go-ieee20305/csipaus"
	"github.com/otfabric/go-ieee20305/tlsccm"

	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// Config is the connection of a client command: the server, the device's
// certificate, and how to speak to the server.
type Config struct {
	URL     string        `flag:"url" env:"IEEE20305CTL_URL" desc:"URL of the server's DeviceCapability, such as https://utility.example/dcap (a host[:port] means https://host[:port]/dcap)"`
	Cert    string        `flag:"cert" env:"IEEE20305CTL_CERT" desc:"Device certificate (PEM)"`
	Key     string        `flag:"key" env:"IEEE20305CTL_KEY" desc:"Private key of the device certificate (PEM)"`
	CA      string        `flag:"ca" env:"IEEE20305CTL_CA" desc:"CA certificates that verify the server (PEM); required, IEEE 2030.5 servers are not verified against the system roots"`
	CCM8    bool          `flag:"ccm8" env:"IEEE20305CTL_CCM8" desc:"Offer TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8 first (needed by servers that offer nothing else, such as GridAPPS-D)"`
	Profile string        `flag:"profile" env:"IEEE20305CTL_PROFILE" desc:"Document profile: ieee20305 or csipaus (CSIP-AUS types and site limits)"`
	Edition string        `flag:"edition" env:"IEEE20305CTL_EDITION" desc:"Edition the device speaks: 2023 or 2018"`
	Timeout time.Duration `flag:"timeout" env:"IEEE20305CTL_TIMEOUT" desc:"Time limit of each request"`
}

// Defaults returns the defaults of a Config.
func Defaults() Config {
	return Config{Profile: "ieee20305", Edition: "2023", Timeout: 30 * time.Second}
}

// Device is a connected device: its client and identity.
type Device struct {
	Client   *ieeeclient.Client
	Identity ieeeclient.Identity
	Cert     tls.Certificate
	Roots    *x509.CertPool
	Codec    ieee20305.Codec
	Edition  ieee20305.Edition
	URL      string
	// HTTP is the HTTP client of the device's certificate, for a second
	// client of the same device (another codec, say).
	HTTP *http.Client
}

// Validate checks c without touching the network or the files.
func (c Config) Validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--url is required (or IEEE20305CTL_URL)", nil)
	}
	if _, err := ServerURL(c.URL); err != nil {
		return err
	}
	if c.Cert == "" || c.Key == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--cert and --key are required: IEEE 2030.5 servers know a device by its certificate", nil)
	}
	if c.CA == "" {
		return errs.InvalidInput(errs.CodeInvalidInput, "--ca is required: the CA certificates that verify the server", nil)
	}
	if _, err := c.edition(); err != nil {
		return err
	}
	switch c.Profile {
	case "ieee20305", "csipaus":
	default:
		return errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--profile %q: use ieee20305 or csipaus", c.Profile), nil)
	}
	if c.Timeout <= 0 {
		return errs.InvalidInput(errs.CodeInvalidInput, "--timeout must be positive", nil)
	}
	return nil
}

func (c Config) edition() (ieee20305.Edition, error) {
	e, err := ieee20305.ParseEdition(c.Edition)
	if err != nil {
		return 0, errs.InvalidInput(errs.CodeInvalidInput, fmt.Sprintf("--edition %q: use 2023 or 2018", c.Edition), err)
	}
	return e, nil
}

// ServerURL turns --url into the URL of a DeviceCapability.
func ServerURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		host := s
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, "443")
		}
		return "https://" + host + "/dcap", nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", errs.InvalidInput(errs.CodeInvalidURL, fmt.Sprintf("--url %q: an https URL, or host[:port]", s), err)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/dcap"
	}
	return u.String(), nil
}

// Connect builds the client c describes.
func (c Config) Connect() (*Device, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	server, _ := ServerURL(c.URL)
	cert, err := LoadKeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, err
	}
	roots, err := LoadPool(c.CA)
	if err != nil {
		return nil, err
	}
	edition, _ := c.edition()
	codec := edition.Codec()
	if c.Profile == "csipaus" {
		codec = csipaus.Codec{}
	}
	hc := ieeeclient.NewHTTPClient(cert, roots)
	if c.CCM8 {
		hc = tlsccm.NewHTTPClient(cert, roots, tlsccm.Options{AllowGCM: true})
	}
	hc.Timeout = c.Timeout
	cl, err := ieeeclient.New(ieeeclient.Config{Server: server, HTTPClient: hc, Codec: codec, UserAgent: "ieee20305ctl"})
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInvalidURL, err.Error(), err)
	}
	return &Device{Client: cl, Identity: ieeeclient.IdentityOf(cert.Leaf), Cert: cert, Roots: roots,
		Codec: codec, Edition: edition, URL: server, HTTP: hc}, nil
}

// LoadKeyPair loads a certificate and its key.
func LoadKeyPair(certFile, keyFile string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("the certificate %s and key %s: %v", certFile, keyFile, err), err)
	}
	if cert.Leaf == nil {
		if cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return tls.Certificate{}, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("the certificate %s: %v", certFile, err), err)
		}
	}
	return cert, nil
}

// LoadPool loads the CA certificates of a PEM file.
func LoadPool(file string) (*x509.CertPool, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInputFileOpenFailed, fmt.Sprintf("the CA certificates %s: %v", file, err), err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("%s holds no PEM certificate", file), nil)
	}
	return pool, nil
}

// LoadCertificate loads the first certificate of a PEM file.
func LoadCertificate(file string) (*x509.Certificate, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errs.InvalidInput(errs.CodeInputFileOpenFailed, fmt.Sprintf("the certificate %s: %v", file, err), err)
	}
	for {
		var b *pem.Block
		b, data = pem.Decode(data)
		if b == nil {
			return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("%s holds no PEM certificate", file), nil)
		}
		if b.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(b.Bytes)
			if err != nil {
				return nil, errs.InvalidInput(errs.CodeInvalidCertificate, fmt.Sprintf("the certificate %s: %v", file, err), err)
			}
			return c, nil
		}
	}
}

// HTTPDetail is the ieee20305 object of a JSON error for an answer of a
// server: see docs/json-output.md.
type HTTPDetail struct {
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	Status     int    `json:"status,omitempty"`
	ReasonCode *int   `json:"reason_code,omitempty"`
	RetryAfter string `json:"retry_after,omitempty"`
}

// Classify maps what a call to a server returned to a typed error of the
// tool. write says whether the call may have changed the server, which turns
// a timeout into an unknown outcome.
func Classify(err error, write bool) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	var status *ieee20305.StatusError
	if strings.Contains(err.Error(), "tls: handshake failure") || strings.Contains(err.Error(), "remote error: tls:") {
		return errs.New(errs.KindTransport, errs.CodeTransportTLSFailed,
			"the TLS handshake failed: "+err.Error()+" (the server may refuse the certificate, or offer only CCM_8: try --ccm8)", err)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return errs.New(errs.KindInterrupted, errs.CodeContextCanceled, "interrupted", err)
	case errors.As(err, &status):
		d := HTTPDetail{Method: status.Method, Path: status.Path, Status: status.StatusCode}
		if status.Document != nil {
			rc := int(status.Document.ReasonCode)
			d.ReasonCode = &rc
			if status.Document.MaxRetryDuration != nil {
				d.RetryAfter = (time.Duration(*status.Document.MaxRetryDuration) * time.Second).String()
			}
		}
		if status.RetryAfter > 0 {
			d.RetryAfter = status.RetryAfter.String()
		}
		code := errs.CodeHTTPError
		switch {
		case errors.Is(err, ieee20305.ErrNotFound):
			code = errs.CodeNotFound
		case errors.Is(err, ieee20305.ErrForbidden):
			code = errs.CodeForbidden
		case errors.Is(err, ieee20305.ErrBadRequest):
			code = errs.CodeBadRequest
		case errors.Is(err, ieee20305.ErrConflict):
			code = errs.CodeConflict
		case errors.Is(err, ieee20305.ErrUnavailable):
			code = errs.CodeUnavailable
		case errors.Is(err, ieee20305.ErrServer):
			code = errs.CodeServerError
		}
		return errs.New(errs.KindNegative, code, fmt.Sprintf("%s %s: the server answered %d %s", status.Method, status.Path,
			status.StatusCode, http.StatusText(status.StatusCode)), err).WithDetail(d)
	case errors.Is(err, ieeeclient.ErrPINMismatch):
		return errs.New(errs.KindProtocol, errs.CodePINMismatch, err.Error(), err)
	case errors.Is(err, ieeeclient.ErrNotRegistered):
		return errs.New(errs.KindProtocol, errs.CodeNotRegistered, err.Error(), err)
	case errors.Is(err, ieee20305.ErrCertificate):
		return errs.New(errs.KindTransport, errs.CodeTransportTLSFailed, "the TLS handshake failed: "+err.Error(), err)
	case errors.Is(err, ieee20305.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		if write {
			return errs.New(errs.KindTimeout, errs.CodeOutcomeUnknown, "no answer in time; the server may have applied the request: "+err.Error(), err)
		}
		return errs.New(errs.KindTimeout, errs.CodeRequestTimeout, "no answer in time: "+err.Error(), err)
	case errors.Is(err, ieee20305.ErrUnavailable):
		return errs.New(errs.KindTransport, errs.CodeTransportConnectFailed, err.Error(), err)
	case errors.Is(err, ieee20305.ErrProtocol), errors.Is(err, ieee20305.ErrInvalid):
		return errs.New(errs.KindProtocol, errs.CodeInvalidResponse, err.Error(), err)
	}
	var tlsErr *tls.CertificateVerificationError
	var opErr *net.OpError
	switch {
	case errors.As(err, &tlsErr):
		return errs.New(errs.KindTransport, errs.CodeTransportTLSFailed, "the TLS handshake failed: "+err.Error(), err)
	case errors.As(err, &opErr):
		return errs.New(errs.KindTransport, errs.CodeTransportConnectFailed, err.Error(), err)
	}
	return errs.New(errs.KindTransport, errs.CodeTransportConnectFailed, err.Error(), err)
}

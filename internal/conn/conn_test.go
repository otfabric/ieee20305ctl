// SPDX-License-Identifier: MIT

package conn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"

	"github.com/otfabric/ieee20305ctl/internal/errs"
)

func TestClassify(t *testing.T) {
	rc := ieee20305.UInt16(3)
	_ = rc
	for _, c := range []struct {
		err   error
		write bool
		kind  errs.Kind
		code  string
	}{
		{&ieee20305.StatusError{Method: "GET", Path: "/x", StatusCode: 404}, false, errs.KindNegative, errs.CodeNotFound},
		{&ieee20305.StatusError{Method: "PUT", Path: "/x", StatusCode: 403}, false, errs.KindNegative, errs.CodeForbidden},
		{&ieee20305.StatusError{Method: "PUT", Path: "/x", StatusCode: 400, Document: &ieee20305.Error{ReasonCode: 1}}, false, errs.KindNegative, errs.CodeBadRequest},
		{&ieee20305.StatusError{Method: "POST", Path: "/x", StatusCode: 409}, false, errs.KindNegative, errs.CodeConflict},
		{&ieee20305.StatusError{Method: "GET", Path: "/x", StatusCode: 503, RetryAfter: 5e9}, false, errs.KindNegative, errs.CodeUnavailable},
		{&ieee20305.StatusError{Method: "GET", Path: "/x", StatusCode: 500}, false, errs.KindNegative, errs.CodeServerError},
		{&ieee20305.StatusError{Method: "GET", Path: "/x", StatusCode: 418}, false, errs.KindNegative, errs.CodeHTTPError},
		{fmt.Errorf("x: %w", ieeeclient.ErrPINMismatch), false, errs.KindProtocol, errs.CodePINMismatch},
		{ieeeclient.ErrNotRegistered, false, errs.KindProtocol, errs.CodeNotRegistered},
		{fmt.Errorf("%w: bad", ieee20305.ErrCertificate), false, errs.KindTransport, errs.CodeTransportTLSFailed},
		{fmt.Errorf("%w: slow", ieee20305.ErrTimeout), false, errs.KindTimeout, errs.CodeRequestTimeout},
		{fmt.Errorf("%w: slow", ieee20305.ErrTimeout), true, errs.KindTimeout, errs.CodeOutcomeUnknown},
		{fmt.Errorf("%w: down", ieee20305.ErrUnavailable), false, errs.KindTransport, errs.CodeTransportConnectFailed},
		{fmt.Errorf("%w: junk", ieee20305.ErrProtocol), false, errs.KindProtocol, errs.CodeInvalidResponse},
		{context.Canceled, false, errs.KindInternal, errs.CodeContextCanceled},
		{&net.OpError{Op: "dial", Err: errors.New("refused")}, false, errs.KindTransport, errs.CodeTransportConnectFailed},
		{errors.New("other"), false, errs.KindTransport, errs.CodeTransportConnectFailed},
		{fmt.Errorf("%w: remote error: tls: handshake failure", ieee20305.ErrUnavailable), false, errs.KindTransport, errs.CodeTransportTLSFailed},
	} {
		e, ok := errs.As(Classify(c.err, c.write))
		if !ok || e.Kind != c.kind || e.Code != c.code {
			t.Errorf("Classify(%v, %v) = %+v, want %s/%s", c.err, c.write, e, c.kind, c.code)
		}
	}
	if Classify(nil, false) != nil {
		t.Error("Classify(nil)")
	}
	typed := errs.InvalidInput(errs.CodeInvalidInput, "x", nil)
	if Classify(typed, false) != typed {
		t.Error("a typed error is not passed through")
	}
}

func TestConfig(t *testing.T) {
	for _, s := range []string{"utility.example", "utility.example:8443", "https://utility.example", "https://utility.example/sep2/dcap"} {
		u, err := ServerURL(s)
		if err != nil || u[:8] != "https://" {
			t.Errorf("ServerURL(%q) = %q, %v", s, u, err)
		}
	}
	if _, err := ServerURL("ftp://x"); err == nil {
		t.Error("an ftp URL")
	}
	c := Defaults()
	c.URL, c.Cert, c.Key, c.CA = "x", "c", "k", "ca"
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Edition = "2019" }, func(c *Config) { c.Profile = "x" }, func(c *Config) { c.Timeout = 0 },
		func(c *Config) { c.CA = "" }, func(c *Config) { c.Key = "" },
	} {
		bad := c
		mutate(&bad)
		if bad.Validate() == nil {
			t.Errorf("Validate(%+v) succeeded", bad)
		}
	}
	if _, err := LoadKeyPair("missing.pem", "missing.key"); err == nil {
		t.Error("LoadKeyPair of nothing")
	}
	if _, err := LoadPool("missing.pem"); err == nil {
		t.Error("LoadPool of nothing")
	}
	if _, err := LoadCertificate("conn.go"); err == nil {
		t.Error("LoadCertificate of Go source")
	}
}

// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
	ieeeclient "github.com/otfabric/go-ieee20305/client"
	"github.com/otfabric/go-ieee20305/security"
	"github.com/otfabric/ieee20305ctl/internal/conn"
)

// hostile is a mutual-TLS server of the tests that answers what its handler
// writes: a server that lies, stalls, redirects or floods.
type hostile struct {
	pki      string
	url      string
	identity ieeeclient.Identity
	requests atomic.Int64
}

// newHostile generates a PKI, serves h with the PKI's server certificate,
// and points the client commands at it as device 1.
func newHostile(t *testing.T, h func(hs *hostile, w http.ResponseWriter, r *http.Request)) *hostile {
	t.Helper()
	hs := &hostile{pki: filepath.Join(t.TempDir(), "pki")}
	must(t, 0, "cert", "generate", "--out", hs.pki, "--devices", "2")
	cert, err := conn.LoadKeyPair(hs.pki+"/server.pem", hs.pki+"/server.key")
	if err != nil {
		t.Fatal(err)
	}
	dev, err := conn.LoadKeyPair(hs.pki+"/device-1.pem", hs.pki+"/device-1.key")
	if err != nil {
		t.Fatal(err)
	}
	hs.identity = ieeeclient.IdentityOf(dev.Leaf)
	roots, err := conn.LoadPool(hs.pki + "/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.requests.Add(1)
		h(hs, w, r)
	}))
	srv.TLS = security.ServerTLSConfig(cert, roots)
	srv.Config.ErrorLog = quietLogger()
	srv.StartTLS()
	t.Cleanup(srv.Close)
	hs.url = srv.URL + "/dcap"
	t.Setenv("IEEE20305CTL_URL", hs.url)
	t.Setenv("IEEE20305CTL_CERT", hs.pki+"/device-1.pem")
	t.Setenv("IEEE20305CTL_KEY", hs.pki+"/device-1.key")
	t.Setenv("IEEE20305CTL_CA", hs.pki+"/ca.pem")
	return hs
}

// writeDoc writes v as the server's answer.
func writeDoc(t *testing.T, w http.ResponseWriter, v any) {
	data, err := ieee20305.Marshal(v)
	if err != nil {
		t.Error(err)
		return
	}
	w.Header().Set("Content-Type", ieeeclient.MediaType)
	_, _ = w.Write(data)
}

// dcap is a DeviceCapability with an EndDeviceList and the links of extra.
func dcap(extra func(*ieee20305.DeviceCapability)) *ieee20305.DeviceCapability {
	dc := &ieee20305.DeviceCapability{EndDeviceListLink: &ieee20305.EndDeviceListLink{}}
	dc.Href = new("/dcap")
	dc.EndDeviceListLink.Href, dc.EndDeviceListLink.All = "/edev", new(ieee20305.UInt32(1))
	dc.TimeLink = &ieee20305.TimeLink{}
	dc.TimeLink.Href = "/tm"
	if extra != nil {
		extra(dc)
	}
	return dc
}

// endDeviceList is the EndDeviceList of the device, whose EndDevice has an
// FSA list at /fsa.
func (hs *hostile) endDeviceList() *ieee20305.EndDeviceList {
	var ed ieee20305.EndDevice
	ed.Href = new("/edev/1")
	ed.SFDI = ieee20305.SFDIType(hs.identity.SFDI)
	lfdi := ieee20305.HexBinary160(hs.identity.LFDI[:])
	ed.LFDI = &lfdi
	ed.FunctionSetAssignmentsListLink = &ieee20305.FunctionSetAssignmentsListLink{}
	ed.FunctionSetAssignmentsListLink.Href = "/fsa"
	ed.FlowReservationRequestListLink = &ieee20305.FlowReservationRequestListLink{}
	ed.FlowReservationRequestListLink.Href = "/frq"
	ed.SubscriptionListLink = &ieee20305.SubscriptionListLink{}
	ed.SubscriptionListLink.Href = "/sub"
	list := &ieee20305.EndDeviceList{EndDevice: []ieee20305.EndDevice{ed}}
	list.Href = new("/edev")
	list.All, list.Results = 1, 1
	return list
}

// serveBase answers /dcap and /edev; it reports whether it did.
func (hs *hostile) serveBase(t *testing.T, w http.ResponseWriter, r *http.Request, extra func(*ieee20305.DeviceCapability)) bool {
	switch r.URL.Path {
	case "/dcap":
		writeDoc(t, w, dcap(extra))
	case "/edev":
		writeDoc(t, w, hs.endDeviceList())
	default:
		return false
	}
	return true
}

func errorCode(t *testing.T, r result) string {
	t.Helper()
	e := decode[struct {
		Error struct {
			Kind string `json:"kind"`
			Code string `json:"code"`
		} `json:"error"`
	}](t, r)
	return e.Error.Code
}

// mustCode runs args with --format json and fails unless the command exits
// with code and, when want is not empty, the error code want.
func mustCode(t *testing.T, code int, want string, args ...string) result {
	t.Helper()
	r := run(t, append(args, "--format", "json")...)
	if r.code != code {
		t.Fatalf("%s: exit %d, want %d\nstdout: %.2000s\nstderr: %.2000s", strings.Join(args, " "), r.code, code, r.stdout, r.stderr)
	}
	if want != "" {
		if got := errorCode(t, r); got != want {
			t.Fatalf("%s: code %q, want %q\n%.2000s", strings.Join(args, " "), got, want, r.stdout)
		}
	}
	return r
}

// quietLogger discards the errors of a test server: the client hanging up
// is what some tests do.
func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

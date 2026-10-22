// SPDX-License-Identifier: MIT

package client

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
	"github.com/otfabric/ieee20305ctl/internal/errs"
)

// registry is a server with an EndDeviceList that devices register in.
type registry struct {
	mu       sync.Mutex
	devices  []ieee20305.EndDevice
	pin      ieee20305.PINType
	noRegLnk bool
	noList   bool
}

func (g *registry) handler(t *testing.T) http.HandlerFunc {
	write := func(w http.ResponseWriter, v any) {
		data, err := ieee20305.Marshal(v)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/sep+xml")
		_, _ = w.Write(data)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		switch p := r.URL.Path; {
		case p == "/dcap":
			var dc ieee20305.DeviceCapability
			if !g.noList {
				dc.EndDeviceListLink = &ieee20305.EndDeviceListLink{}
				dc.EndDeviceListLink.Href = "/edev"
			}
			write(w, &dc)
		case p == "/edev" && r.Method == http.MethodGet:
			var list ieee20305.EndDeviceList
			for _, ed := range g.devices {
				if s := r.URL.Query().Get("sFDI"); s == "" || s == strconv.FormatUint(uint64(ed.SFDI), 10) {
					list.EndDevice = append(list.EndDevice, ed)
				}
			}
			list.All, list.Results = ieee20305.UInt32(len(list.EndDevice)), ieee20305.UInt32(len(list.EndDevice))
			write(w, &list)
		case p == "/edev" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var ed ieee20305.EndDevice
			if err := ieee20305.Unmarshal(body, &ed); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			href := "/edev/" + strconv.Itoa(len(g.devices))
			ed.Href = &href
			if !g.noRegLnk {
				ed.RegistrationLink = &ieee20305.RegistrationLink{}
				ed.RegistrationLink.Href = href + "/rg"
			}
			g.devices = append(g.devices, ed)
			w.Header().Set("Location", href)
			w.WriteHeader(http.StatusCreated)
		case strings.HasSuffix(p, "/rg"):
			write(w, &ieee20305.Registration{PIN: g.pin, DateTimeRegistered: 1700000000})
		case strings.HasPrefix(p, "/edev/"):
			i, _ := strconv.Atoi(strings.TrimPrefix(p, "/edev/"))
			if i < 0 || i >= len(g.devices) {
				http.NotFound(w, r)
				return
			}
			write(w, &g.devices[i])
		default:
			http.NotFound(w, r)
		}
	}
}

func TestRegister(t *testing.T) {
	p := newTestPKI(t)
	g := &registry{pin: 111115}
	srv := startServer(t, p, g.handler(t))
	flags := p.flags(srv.URL)

	out, err := run(t, append([]string{"register", "--pin", "111115"}, flags...)...)
	if err != nil {
		t.Fatalf("register: %v\n%s", err, out)
	}
	for _, want := range []string{"EndDevice:     /edev/0 (created)", "PIN:         111115 (matches --pin)", "Registered:  2023-11-14T22:13:20Z", "Poll rate:   900 s"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	out, err = run(t, append([]string{"register", "-o", "json"}, flags...)...)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string
		Data   RegisterResult
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	d := env.Data
	if env.Status != "ok" || d.Created || d.EndDevice != "/edev/0" || len(d.LFDI) != 40 || d.SFDI == 0 ||
		d.Registration == nil || d.Registration.PIN != 111115 || d.Registration.PINVerified {
		t.Errorf("second register = %s", out)
	}
	if len(g.devices) != 1 {
		t.Errorf("the server has %d devices, want 1", len(g.devices))
	}
}

func TestRegisterFailures(t *testing.T) {
	p := newTestPKI(t)

	g := &registry{pin: 111115}
	srv := startServer(t, p, g.handler(t))
	out, err := run(t, append([]string{"register", "--pin", "123455", "-o", "json"}, p.flags(srv.URL)...)...)
	if exitCode(err) != errs.ExitError || !strings.Contains(err.Error(), "PIN mismatch") {
		t.Errorf("wrong PIN: %v", err)
	}
	if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, `"endDevice": "/edev/0"`) {
		t.Errorf("a failed JSON result keeps what was found: %s", out)
	}

	empty := startServer(t, p, (&registry{}).handler(t))
	if _, err := run(t, append([]string{"register", "--no-create"}, p.flags(empty.URL)...)...); exitCode(err) != errs.ExitError || !strings.Contains(err.Error(), "no EndDevice") {
		t.Errorf("--no-create on an empty list: %v", err)
	}

	unreg := &registry{noRegLnk: true}
	unregSrv := startServer(t, p, unreg.handler(t))
	out, err = run(t, append([]string{"register"}, p.flags(unregSrv.URL)...)...)
	if err != nil || !strings.Contains(out, "none: the server has not registered the device") {
		t.Errorf("unregistered without --pin: %v\n%s", err, out)
	}
	if _, err := run(t, append([]string{"register", "--pin", "111115"}, p.flags(unregSrv.URL)...)...); exitCode(err) != errs.ExitError {
		t.Errorf("unregistered with --pin: %v", err)
	}

	noList := startServer(t, p, (&registry{noList: true}).handler(t))
	if _, err := run(t, append([]string{"register"}, p.flags(noList.URL)...)...); exitCode(err) != errs.ExitError || !strings.Contains(err.Error(), "EndDeviceList") {
		t.Errorf("no EndDeviceList: %v", err)
	}
	if _, err := run(t, append([]string{"register", "--pin", "0"}, p.flags(srv.URL)...)...); exitCode(err) != errs.ExitUsage {
		t.Errorf("--pin 0: %v", err)
	}
}

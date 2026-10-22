// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

func TestOffline(t *testing.T) {
	must(t, 0, "version")
	if r := must(t, 0, "completion", "bash"); !strings.Contains(r.stdout, "ieee20305ctl") {
		t.Error("no bash completion")
	}
	if r := run(t, "nope"); r.code != 2 || !strings.Contains(r.stderr, "unknown command") {
		t.Errorf("an unknown command: %+v", r)
	}
	if r := run(t, "client", "dcap", "--format", "yaml"); r.code != 3 || !strings.Contains(r.stderr, "invalid output format") {
		t.Errorf("a bad --format: %+v", r)
	}

	dir := t.TempDir()
	gen := decode[struct {
		Files []struct {
			Role string `json:"role"`
			LFDI string `json:"lfdi"`
			SFDI string `json:"sfdi"`
		} `json:"files"`
	}](t, must(t, 0, "cert", "generate", "--out", dir, "--devices", "2", "--hmn", "--format", "json"))
	if len(gen.Files) != 4 || gen.Files[2].Role != "device" || len(gen.Files[2].LFDI) != 40 {
		t.Fatalf("generate: %+v", gen)
	}
	if r := run(t, "cert", "generate", "--out", dir); r.code != 3 {
		t.Errorf("generate over existing files: exit %d", r.code)
	}
	info := must(t, 0, "cert", "inspect", dir+"/device-1.pem").stdout
	for _, want := range []string{"Hardware module", "device-1", gen.Files[2].LFDI, "client auth"} {
		if !strings.Contains(info, want) {
			t.Errorf("inspect lacks %q:\n%s", want, info)
		}
	}
	if r := must(t, 0, "cert", "lfdi", dir+"/device-1.pem", "--format", "table"); !strings.Contains(r.stdout, gen.Files[2].SFDI) {
		t.Errorf("lfdi: %s", r.stdout)
	}
	must(t, 0, "cert", "verify", dir+"/device-1.pem", "--ca", dir+"/ca.pem")
	must(t, 0, "cert", "verify", dir+"/server.pem", "--ca", dir+"/ca.pem", "--usage", "server", "--name", "localhost")
	other := t.TempDir()
	must(t, 0, "cert", "generate", "--out", other)
	if r := run(t, "cert", "verify", dir+"/device-1.pem", "--ca", other+"/ca.pem", "--format", "json"); r.code != 3 ||
		!strings.Contains(r.stdout, `"invalid_certificate"`) {
		t.Errorf("verify against another CA: %+v", r)
	}
	if r := run(t, "cert", "inspect", filepath.Join(dir, "missing.pem")); r.code != 3 {
		t.Errorf("inspect of a missing file: %+v", r)
	}

	// validate: a valid document and a broken one.
	ctl := &ieee20305.DERControl{}
	ctl.MRID = ieee20305.MRIDType{1, 2, 3, 4}
	ctl.Interval = ieee20305.DateTimeInterval{Start: 1700000000, Duration: 600}
	data, err := ieee20305.Marshal(ctl)
	if err != nil {
		t.Fatal(err)
	}
	good, bad := filepath.Join(dir, "good.xml"), filepath.Join(dir, "bad.xml")
	_ = os.WriteFile(good, data, 0o644)
	_ = os.WriteFile(bad, []byte(`<DERControl xmlns="urn:ieee:std:2030.5:ns"><mRID>XYZ</mRID></DERControl>`), 0o644)
	if r := must(t, 0, "validate", good); !strings.Contains(r.stdout, "valid DERControl") {
		t.Errorf("validate: %s", r.stdout)
	}
	if r := run(t, "validate", good, bad, "--format", "json"); r.code != 3 || !strings.Contains(r.stdout, `"invalid": 1`) {
		t.Errorf("validate of an invalid document: %+v", r)
	}
	if r := must(t, 0, "server", "generate-config"); !strings.Contains(r.stdout, "programs:") {
		t.Errorf("generate-config: %s", r.stdout)
	}
	out := filepath.Join(dir, "seed.yaml")
	must(t, 0, "server", "generate-config", "--output", out)
	if r := run(t, "server", "generate-config", "--output", out); r.code != 3 {
		t.Errorf("generate-config over a file: %+v", r)
	}
}

func TestClientAgainstServer(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")

	dcap := decode[struct {
		PollRate uint32 `json:"poll_rate_s"`
		Links    []struct{ Name, Href string }
	}](t, must(t, 0, "client", "dcap", "--format", "json"))
	if dcap.PollRate != 1 || len(dcap.Links) < 5 {
		t.Errorf("dcap: %+v", dcap)
	}
	must(t, 0, "client", "time")
	reg := decode[struct {
		EndDevice  string `json:"end_device"`
		PINMatches *bool  `json:"pin_matches"`
	}](t, must(t, 0, "client", "register", "--pin", "111115", "--format", "json"))
	if reg.EndDevice == "" || reg.PINMatches == nil || !*reg.PINMatches {
		t.Errorf("register: %+v", reg)
	}
	if r := run(t, "client", "register", "--pin", "1", "--format", "json"); r.code != 6 || !strings.Contains(r.stdout, `"pin_mismatch"`) {
		t.Errorf("a wrong PIN: %+v", r)
	}
	if r := must(t, 0, "client", "programs", "--format", "table"); !strings.Contains(r.stdout, "Utility program") || !strings.Contains(r.stdout, "opModMaxLimW=100.00 %") {
		t.Errorf("programs: %s", r.stdout)
	}
	ctls := decode[struct {
		Controls []struct {
			Href, Status, ReplyTo string
			MRID                  string `json:"mrid"`
		} `json:"controls"`
	}](t, must(t, 0, "client", "controls", "--active", "--format", "json"))
	if len(ctls.Controls) != 1 || ctls.Controls[0].Status != "Active" {
		t.Fatalf("active controls: %+v", ctls)
	}
	if r := must(t, 0, "client", "pricing"); !strings.Contains(r.stdout, "0.15/kWh") {
		t.Errorf("pricing: %s", r.stdout)
	}
	must(t, 0, "client", "respond", ctls.Controls[0].Href, "--status", "started")
	if !s.seen(func(e event) bool { return e.Event == "response" && e.Code == 2 && e.MRID == ctls.Controls[0].MRID }) {
		t.Error("the server saw no response")
	}
	if r := run(t, "client", "respond", ctls.Controls[0].Href, "--status", "maybe"); r.code != 3 {
		t.Errorf("a bad status: %+v", r)
	}
	must(t, 0, "client", "report", "--power", "4200")
	if r := run(t, "client", "report"); r.code != 3 {
		t.Errorf("report without --power or --file: %+v", r)
	}
	must(t, 0, "client", "log-event", "--code", "1", "--id", "7", "--details", "over-temperature")
	must(t, 0, "client", "flow-reservation", "--energy", "20000", "--power", "7000", "--start", "1h", "--duration", "4h")
	if r := must(t, 0, "client", "get", reg.EndDevice+"/rg"); !strings.Contains(r.stdout, "<pIN>111115</pIN>") {
		t.Errorf("get: %s", r.stdout)
	}
	walk := events(t, must(t, 0, "client", "walk", "--depth", "1", "--format", "json").stdout)
	if !has(walk, func(e event) bool { return e.Href == "/edev" }) {
		t.Errorf("walk: %+v", walk)
	}

	// DER resources: none, then put from files.
	if r := must(t, 0, "client", "der", "show"); !strings.Contains(r.stdout, "not set") {
		t.Errorf("der show: %s", r.stdout)
	}
	settings := filepath.Join(s.dir, "derg.xml")
	data, _ := ieee20305.Marshal(&ieee20305.DERSettings{SetMaxW: ieee20305.ActivePower{Value: 5000}, SetGradW: 100, UpdatedTime: 1700000000})
	_ = os.WriteFile(settings, data, 0o644)
	must(t, 0, "client", "der", "put", "--settings", settings)
	if r := must(t, 0, "client", "der", "show", "--xml"); !strings.Contains(r.stdout, "<setMaxW>") {
		t.Errorf("der show after put: %s", r.stdout)
	}
	must(t, 0, "client", "put", reg.EndDevice+"/der/1/derg", "--file", settings)
	logEvent := filepath.Join(s.dir, "le.xml")
	data, _ = ieee20305.Marshal(&ieee20305.LogEvent{CreatedDateTime: 1700000000, LogEventCode: 0, LogEventID: 9, FunctionSet: 1})
	_ = os.WriteFile(logEvent, data, 0o644)
	if r := must(t, 0, "client", "post", reg.EndDevice+"/lel", "--file", logEvent); !strings.Contains(r.stdout, "created") {
		t.Errorf("post: %s", r.stdout)
	}
	if r := run(t, "client", "delete", reg.EndDevice+"/lel/1"); r.code != 2 {
		t.Errorf("delete without --yes: %+v", r)
	}
	if r := run(t, "client", "connection-point"); r.code != 6 {
		t.Errorf("connection-point of a server without CSIP-AUS: %+v", r)
	}

	// The operator.
	devs := decode[struct {
		Devices []struct{ LFDI string } `json:"devices"`
	}](t, must(t, 0, "server", "devices", "--format", "json"))
	if len(devs.Devices) != 1 {
		t.Errorf("devices: %+v", devs)
	}
	if r := run(t, "server", "issue-control", "--set", "opModExpLimW=1500"); r.code != 2 {
		t.Errorf("issue-control without --yes: %+v", r)
	}
	if r := run(t, "server", "issue-control", "--set", "opModExpLimW=1500", "--yes"); r.code != 3 || !strings.Contains(r.stderr, "--profile csipaus") {
		t.Errorf("a CSIP-AUS setting on a server of IEEE 2030.5: %+v", r)
	}
	if r := run(t, "server", "issue-control", "--set", "opModNothing=1", "--yes"); r.code != 3 {
		t.Errorf("issue-control of an unknown control: %+v", r)
	}
	issued := decode[struct{ Href string }](t, must(t, 0, "server", "issue-control", "--program", "Utility program",
		"--set", "opModMaxLimW=30", "--duration", "10m", "--yes", "--format", "json"))
	if r := must(t, 0, "client", "controls", "--active"); !strings.Contains(r.stdout, "opModMaxLimW=30.00 %") {
		t.Errorf("the issued control: %s", r.stdout)
	}
	must(t, 0, "server", "cancel", issued.Href, "--yes")
	must(t, 0, "server", "set-default", "--set", "opModMaxLimW=80", "--yes")
	if r := must(t, 0, "server", "get", "/derp/1/dderc"); !strings.Contains(r.stdout, "<opModMaxLimW>8000</opModMaxLimW>") {
		t.Errorf("get of the default: %s", r.stdout)
	}
	if r := run(t, "server", "get", "/nope"); r.code != 3 {
		t.Errorf("get of nothing: %+v", r)
	}

	// Device 2: unknown, then provisioned.
	s.device(t, "2")
	if r := run(t, "client", "register", "--format", "json"); r.code != 6 || !strings.Contains(r.stdout, `"status": 403`) {
		t.Errorf("an unprovisioned device: %+v", r)
	}
	must(t, 0, "server", "provision", "--cert", s.pki+"/device-2.pem", "--pin", "222226")
	must(t, 0, "client", "register", "--pin", "222226")
}

func TestRunAndSubscribe(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	r := must(t, 0, "client", "run", "--pin", "111115", "--poll-rate", "1s", "--report-power", "4200", "--post-rate", "1s",
		"--duration", "3s", "--format", "json")
	evs := events(t, r.stdout)
	for _, want := range []func(event) bool{
		func(e event) bool { return e.Event == "started" },
		func(e event) bool { return e.Event == "der_control" && e.Status == "Active" && len(e.Settings) > 0 },
		func(e event) bool { return e.Event == "default_control" },
		func(e event) bool { return e.Event == "response" && e.Code == 2 },
		func(e event) bool { return e.Event == "price" },
	} {
		if !has(evs, want) {
			t.Errorf("client run wrote no such event: %s", r.stdout)
		}
	}
	if r := run(t, "client", "run", "--pin", "1", "--duration", "5s"); r.code != 6 {
		t.Errorf("client run with a wrong PIN: %+v", r)
	}

	done := make(chan result, 1)
	go func() {
		done <- run(t, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--count", "1", "--duration", "20s", "--format", "json")
	}()
	if !s.seen(func(e event) bool { return e.Event == "log" && strings.Contains(e.Message, "subscription") }) {
		t.Fatal("no subscription")
	}
	must(t, 0, "server", "issue-control", "--set", "opModConnect=false", "--start", "1h", "--yes")
	sub := <-done
	if sub.code != 0 {
		t.Fatalf("subscribe: %+v", sub)
	}
	got := events(t, sub.stdout)
	if !has(got, func(e event) bool { return e.Event == "notification" && e.Resource == "/derp/1/derc" }) ||
		!has(got, func(e event) bool { return e.Event == "unsubscribed" }) {
		t.Errorf("subscribe: %s", sub.stdout)
	}
}

func TestCSIPAUS(t *testing.T) {
	s := startSite(t, `devices:
  - cert: %PKI%/device-1.pem
programs:
  - description: Site limits
    primacy: 1
    default:
      opModExpLimW: 5000
      opModImpLimW: 5000
    controls:
      - description: Envelope
        set:
          opModExpLimW: 1500
        duration: 30m
`, "--profile", "csipaus")
	s.device(t, "1")
	t.Setenv("IEEE20305CTL_PROFILE", "csipaus")
	must(t, 0, "client", "register")
	if r := must(t, 0, "client", "programs"); !strings.Contains(r.stdout, "opModExpLimW=5000 W") {
		t.Errorf("default limits: %s", r.stdout)
	}
	if r := must(t, 0, "client", "controls", "--active"); !strings.Contains(r.stdout, "opModExpLimW=1500 W") {
		t.Errorf("envelope: %s", r.stdout)
	}
	if r := must(t, 0, "client", "connection-point"); !strings.Contains(r.stdout, "not set") {
		t.Errorf("connection-point: %s", r.stdout)
	}
	cp := decode[struct {
		ID string `json:"connection_point_id"`
	}](t, must(t, 0, "client", "connection-point", "--set", "4407123456", "--format", "json"))
	if cp.ID != "4407123456" {
		t.Errorf("connection-point --set: %+v", cp)
	}
	must(t, 0, "server", "issue-control", "--set", "opModGenLimW=3000", "--yes")
}

func TestErrors(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	t.Setenv("IEEE20305CTL_URL", "")
	if r := run(t, "client", "dcap"); r.code != 3 || !strings.Contains(r.stderr, "--url is required") {
		t.Errorf("no --url: %+v", r)
	}
	if r := run(t, "client", "dcap", "--url", "http://127.0.0.1/dcap"); r.code != 3 {
		t.Errorf("an http URL: %+v", r)
	}
	if r := run(t, "client", "dcap", "--url", "127.0.0.1:1", "--format", "json"); r.code != 4 || !strings.Contains(r.stdout, `"transport"`) {
		t.Errorf("a refused connection: %+v", r)
	}
	other := t.TempDir()
	must(t, 0, "cert", "generate", "--out", other)
	if r := run(t, "client", "dcap", "--url", s.url, "--ca", other+"/ca.pem", "--format", "json"); r.code != 4 || !strings.Contains(r.stdout, "transport_tls_failed") {
		t.Errorf("a server of another CA: %+v", r)
	}
	if r := run(t, "client", "dcap", "--url", s.url, "--cert", other+"/device-1.pem", "--key", other+"/device-1.key"); r.code != 4 && r.code != 6 {
		t.Errorf("a device of another CA: %+v", r)
	}
	if r := run(t, "client", "get", "/nothing", "--url", s.url, "--format", "json"); r.code != 6 || !strings.Contains(r.stdout, `"not_found"`) {
		t.Errorf("a missing resource: %+v", r)
	}
	if r := run(t, "server", "devices", "--control-socket", filepath.Join(other, "none.sock")); r.code != 4 {
		t.Errorf("no control socket: %+v", r)
	}
	if r := run(t, "server", "start", "--cert", "x"); r.code != 3 {
		t.Errorf("server start without --key and --ca: %+v", r)
	}
	if r := run(t, "server", "start", "--cert", s.pki+"/server.pem", "--key", s.pki+"/server.key", "--ca", s.pki+"/ca.pem",
		"--listen", strings.TrimPrefix(strings.TrimSuffix(s.url, "/dcap"), "https://")); r.code != 4 {
		t.Errorf("server start on a used port: %+v", r)
	}
	bad := filepath.Join(other, "bad.yaml")
	_ = os.WriteFile(bad, []byte("programs:\n  - primacy: 1\n"), 0o644)
	if r := run(t, "server", "start", "--cert", s.pki+"/server.pem", "--key", s.pki+"/server.key", "--ca", s.pki+"/ca.pem",
		"--listen", "127.0.0.1:0", "--config", bad); r.code != 3 || !strings.Contains(r.stderr, "description is required") {
		t.Errorf("a bad seed: %+v", r)
	}
}

// Every command that reads renders in each format.
func TestFormats(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	must(t, 0, "client", "register", "--pin", "111115")
	must(t, 0, "client", "report", "--power", "1000")
	for _, args := range [][]string{
		{"client", "dcap"}, {"client", "time"}, {"client", "register"}, {"client", "programs"}, {"client", "controls"},
		{"client", "pricing"}, {"client", "der", "show"}, {"client", "walk", "--depth", "1"},
		{"client", "log-event", "--id", "1"}, {"client", "report", "--power", "-200000"},
		{"client", "flow-reservation", "--energy", "100000000000000", "--power", "100000"},
		{"server", "devices"}, {"server", "set-default", "--set", "opModConnect=true", "--yes"},
		{"server", "issue-control", "--set", "opModFixedW=-10", "--duration", "1m", "--yes"},
		{"cert", "inspect", s.pki + "/server.pem"}, {"cert", "lfdi", s.pki + "/ca.pem"},
		{"cert", "verify", s.pki + "/device-2.pem", "--ca", s.pki + "/ca.pem"},
		{"validate", s.dir + "/seed.yaml"},
	} {
		for _, f := range []string{"text", "table", "json"} {
			r := run(t, append(args, "--format", f)...)
			want := 0
			if args[0] == "validate" {
				want = 3 // the seed is not an IEEE 2030.5 document
			}
			if r.code != want || r.stdout == "" {
				t.Errorf("%v --format %s: exit %d\n%s%s", args, f, r.code, r.stdout, r.stderr)
			}
		}
	}
	if r := run(t, "client", "flow-reservation", "--power", "100001"); r.code != 3 {
		t.Errorf("a power that does not fit: %+v", r)
	}
}

// A device of the seed that is not registered yet, then registered by the
// operator; in-band registration of an unknown device.
func TestRegistration(t *testing.T) {
	s := startSite(t, `devices:
  - cert: %PKI%/device-1.pem
    unregistered: true
    edition: "2018"
  - lfdi: 0000000000000000000000000000000000000A01
    role: operator
  - lfdi: 0000000000000000000000000000000000000A02
    role: aggregator
  - lfdi: 0000000000000000000000000000000000000A03
    managed_by: 0000000000000000000000000000000000000A02
    device_category: "00020000"
    pin: 4
programs:
  - description: Only program
    primacy: 1
`, "--in-band", "--log-requests")
	s.device(t, "1")
	if r := run(t, "client", "register", "--format", "json"); r.code != 0 || !strings.Contains(r.stdout, `"registered": false`) {
		t.Errorf("an unregistered device: %+v", r)
	}
	devs := decode[struct {
		Devices []struct{ LFDI string } `json:"devices"`
	}](t, must(t, 0, "server", "devices", "--format", "json"))
	must(t, 0, "server", "register", "--lfdi", devs.Devices[0].LFDI, "--pin", "333333")
	must(t, 0, "client", "register", "--pin", "333333")
	if r := run(t, "server", "register", "--lfdi", "nope"); r.code != 3 {
		t.Errorf("register of a bad LFDI: %+v", r)
	}
	// In-band: device 2 posts its EndDevice.
	s.device(t, "2")
	if r := must(t, 0, "client", "register", "--format", "json"); !strings.Contains(r.stdout, `"created": true`) {
		t.Errorf("in-band registration: %s", r.stdout)
	}
	if r := run(t, "client", "register", "--no-create"); r.code != 0 {
		t.Errorf("register --no-create of a known device: %+v", r)
	}
	if !s.seen(func(e event) bool { return e.Event == "request" && e.Code == 201 }) {
		t.Error("no request records with --log-requests")
	}
	if r := must(t, 0, "client", "controls", "--program", "Only program", "--format", "table"); !strings.Contains(r.stdout, "HREF") {
		t.Errorf("controls of a program: %s", r.stdout)
	}
	if r := run(t, "client", "der", "put", "--settings", "x.xml", "--der", "/nope"); r.code != 6 && r.code != 3 {
		t.Errorf("der put to no DER: %+v", r)
	}
	if r := run(t, "client", "der", "put"); r.code != 3 {
		t.Errorf("der put of nothing: %+v", r)
	}
	if r := run(t, "client", "put", "/edev/2/di"); r.code != 3 {
		t.Errorf("put without --file: %+v", r)
	}
	if r := run(t, "client", "post", "/edev/2/lel", "--file", s.dir+"/seed.yaml"); r.code != 3 {
		t.Errorf("post of a file that is no document: %+v", r)
	}
	if r := run(t, "client", "respond", "/dcap"); r.code != 3 {
		t.Errorf("respond to the DeviceCapability: %+v", r)
	}
	if r := run(t, "server", "provision", "--lfdi", devs.Devices[0].LFDI, "--cert", s.pki+"/device-1.pem"); r.code != 3 {
		t.Errorf("provision with --cert and --lfdi: %+v", r)
	}
}

func TestCompletionAndDiscover(t *testing.T) {
	if r := must(t, 0, "__complete", "client", "dcap", "--format", ""); !strings.Contains(r.stdout, "json") {
		t.Errorf("completion of --format: %s", r.stdout)
	}
	for _, sh := range []string{"zsh", "fish", "powershell"} {
		must(t, 0, "completion", sh)
	}
	// Browsing needs multicast, which a CI runner may not have.
	if r := run(t, "client", "discover", "--wait", "300ms", "--format", "json"); r.code != 0 && r.code != 4 {
		t.Errorf("discover: %+v", r)
	}
	if r := run(t, "client", "discover", "--wait", "0s"); r.code != 3 {
		t.Errorf("discover --wait 0: %+v", r)
	}
	t.Setenv("IEEE20305CTL_TIMEOUT", "soon")
	if r := run(t, "version"); r.code != 3 {
		t.Errorf("a malformed environment variable: %+v", r)
	}
}

func TestSubscriptionKept(t *testing.T) {
	s := startSite(t, "")
	s.device(t, "1")
	r := must(t, 0, "client", "subscribe", "/derp/1/derc", "--listen", "127.0.0.1:0", "--keep", "--duration", "1s", "--format", "json")
	var sub string
	for _, e := range events(t, r.stdout) {
		if e.Event == "subscribed" {
			sub = e.Href
		}
		if e.Event == "unsubscribed" {
			t.Error("--keep unsubscribed")
		}
	}
	must(t, 0, "client", "delete", sub, "--yes")
	if r := run(t, "client", "subscribe", "/x", "--listen", "nonsense"); r.code != 3 {
		t.Errorf("subscribe --listen nonsense: %+v", r)
	}
	if r := run(t, "client", "subscribe", "/x", "--notify-url", "http://x"); r.code != 3 {
		t.Errorf("subscribe with an http --notify-url: %+v", r)
	}
	if r := run(t, "client", "run", "--notify-listen", "0.0.0.0:0"); r.code != 3 {
		t.Errorf("run --notify-listen 0.0.0.0 without --notify-url: %+v", r)
	}
	// The agent subscribes, through its listener, and polls less.
	r = must(t, 0, "client", "run", "--notify-listen", "127.0.0.1:0", "--duration", "2s", "--format", "json")
	if !has(events(t, r.stdout), func(e event) bool { return e.Event == "listening" }) {
		t.Errorf("client run --notify-listen: %s", r.stdout)
	}
}

// CCM_8 on both ends, and the server advertised with DNS-SD.
func TestCCM8(t *testing.T) {
	s := startSite(t, "", "--ccm8", "--advertise")
	s.device(t, "1")
	if r := must(t, 0, "client", "dcap", "--ccm8"); !strings.Contains(r.stdout, "EndDeviceListLink") {
		t.Errorf("dcap over CCM_8: %s", r.stdout)
	}
	// The listener offers AES-128-GCM after CCM_8, for crypto/tls.
	must(t, 0, "client", "time")
	if !s.seen(func(e event) bool { return e.Event == "advertised" || e.Event == "warning" }) {
		t.Error("no record of the DNS-SD advertisement")
	}
	// The ResponseList a device answered reads back (go-ieee20305 v0.5.1).
	must(t, 0, "client", "respond", "/derp/1/derc/1", "--status", "received")
	if r := must(t, 0, "client", "walk", "--depth", "3", "--format", "json"); !strings.Contains(r.stdout, `"/rsps/1/rsp"`) {
		t.Errorf("walk: %s", r.stdout)
	}
}

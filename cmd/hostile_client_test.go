// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// A redirect to another host is not followed: the device's certificate and
// requests go only to --url.
func TestHostileRedirectOffServer(t *testing.T) {
	var elsewhere atomic.Int64
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere.Add(1) }))
	other.Config.ErrorLog = quietLogger()
	t.Cleanup(other.Close)
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			newHostile(t, func(_ *hostile, w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, other.URL+r.URL.Path, status)
			})
			r := run(t, "client", "dcap", "--format", "json")
			if r.code != 4 && r.code != 6 {
				t.Fatalf("a redirect off the server: exit %d, want 4 or 6\n%s", r.code, r.stdout)
			}
			if n := elsewhere.Load(); n != 0 {
				t.Fatalf("the client followed the redirect: %d requests to the other host", n)
			}
		})
	}
}

// A redirect loop on the server itself ends.
func TestHostileRedirectLoop(t *testing.T) {
	hs := newHostile(t, func(_ *hostile, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dcap?again="+r.URL.Query().Get("again")+"x", http.StatusFound)
	})
	r := run(t, "client", "dcap", "--format", "json")
	if r.code != 4 && r.code != 6 {
		t.Fatalf("a redirect loop: exit %d\n%s", r.code, r.stdout)
	}
	if n := hs.requests.Load(); n > 20 {
		t.Fatalf("%d requests for a redirect loop", n)
	}
}

// A body beyond the client's limit is refused as an invalid answer, and is
// not read to its end.
func TestHostileHugeBody(t *testing.T) {
	var sent atomic.Int64
	newHostile(t, func(_ *hostile, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sep+xml")
		_, _ = w.Write([]byte(`<DeviceCapability xmlns="urn:ieee:std:2030.5:ns" href="/dcap">`))
		chunk := bytes.Repeat([]byte("<!-- padding -->"), 4096)
		for range 1024 { // 64 MiB
			n, err := w.Write(chunk)
			sent.Add(int64(n))
			if err != nil || r.Context().Err() != nil {
				return
			}
		}
	})
	mustCode(t, 6, "invalid_response", "client", "dcap")
	if n := sent.Load(); n >= 64<<20 {
		t.Fatalf("the client read all %d bytes of an oversized body", n)
	}
}

// A body that is not a document of the schema exits 6.
func TestHostileGarbageBodies(t *testing.T) {
	for name, body := range map[string]string{
		"html":       "<html><body>hello</body></html>",
		"empty":      "",
		"truncated":  `<DeviceCapability xmlns="urn:ieee:std:2030.5:ns" href="/dcap"><EndDeviceListLink href="/ed`,
		"wrong-root": `<Time xmlns="urn:ieee:std:2030.5:ns"><currentTime>1</currentTime></Time>`,
		"entity":     `<!DOCTYPE x [<!ENTITY a "aaaaaaaaaa"><!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">]><DeviceCapability xmlns="urn:ieee:std:2030.5:ns" href="&b;"/>`,
		"deep":       strings.Repeat("<a>", 100000) + strings.Repeat("</a>", 100000),
	} {
		t.Run(name, func(t *testing.T) {
			newHostile(t, func(_ *hostile, w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/sep+xml")
				_, _ = w.Write([]byte(body))
			})
			mustCode(t, 6, "invalid_response", "client", "dcap")
		})
	}
}

// A server that answers the headers and then stalls the body, or never
// answers, makes a read-only command exit 5 with request_timeout, and a
// state-changing one with outcome_unknown.
func TestHostileStall(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run("partial="+strconv.FormatBool(partial), func(t *testing.T) {
			release := make(chan struct{})
			defer close(release) // before the server's Close, which waits for the handlers
			newHostile(t, func(hs *hostile, w http.ResponseWriter, r *http.Request) {
				if hs.serveBase(t, w, r, nil) {
					return
				}
				if partial {
					w.Header().Set("Content-Type", "application/sep+xml")
					_, _ = w.Write([]byte(`<Time xmlns="urn:ieee:std:2030.5:ns">`))
					w.(http.Flusher).Flush()
				}
				select {
				case <-release:
				case <-r.Context().Done():
				}
			})
			mustCode(t, 5, "request_timeout", "client", "time", "--timeout", "1s")
			mustCode(t, 5, "outcome_unknown", "client", "flow-reservation", "--energy", "1000", "--power", "1000", "--timeout", "1s")
		})
	}
}

// A connection cut in the middle of an answer is a transport failure, or an
// answer that does not decode; never a success or an internal error.
func TestHostileConnectionCut(t *testing.T) {
	newHostile(t, func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		w.Header().Set("Content-Type", "application/sep+xml")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<Time xmlns="urn:ieee:std:2030.5:ns"><currentTime>1</cur`))
		w.(http.Flusher).Flush()
		c, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			_ = c.Close()
		}
	})
	r := run(t, "client", "time", "--format", "json")
	if r.code != 4 && r.code != 6 {
		t.Fatalf("a connection cut mid-body: exit %d\n%s", r.code, r.stdout)
	}
}

// A 503 with an absurd Retry-After does not make a one-shot command wait:
// it exits 6 with the status, and the retry_after it reports is bounded.
func TestHostileRetryAfter(t *testing.T) {
	for _, ra := range []string{"999999999999", "-5", "Fri, 31 Dec 9999 23:59:59 GMT", "garbage"} {
		t.Run(ra, func(t *testing.T) {
			newHostile(t, func(_ *hostile, w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", ra)
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			r := mustCode(t, 6, "unavailable", "client", "dcap")
			if strings.Contains(r.stdout, `"retry_after": "-`) {
				t.Errorf("a negative retry_after: %s", r.stdout)
			}
		})
	}
}

// farTimes is a server whose control and tariff interval start at times
// that are not years 0-9999, with durations at their limit.
func farTimes(t *testing.T, start ieee20305.TimeType) func(hs *hostile, w http.ResponseWriter, r *http.Request) {
	return func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		switch r.URL.Path {
		case "/fsa":
			var f ieee20305.FunctionSetAssignments
			f.Href = new("/fsa/1")
			f.DERProgramListLink = &ieee20305.DERProgramListLink{}
			f.DERProgramListLink.Href = "/derp"
			f.TariffProfileListLink = &ieee20305.TariffProfileListLink{}
			f.TariffProfileListLink.Href = "/tp"
			l := &ieee20305.FunctionSetAssignmentsList{FunctionSetAssignments: []ieee20305.FunctionSetAssignments{f}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/derp":
			var p ieee20305.DERProgram
			p.Href = new("/derp/1")
			p.MRID = ieee20305.MRIDType{1}
			p.DERControlListLink = &ieee20305.DERControlListLink{}
			p.DERControlListLink.Href = "/derp/1/derc"
			l := &ieee20305.DERProgramList{DERProgram: []ieee20305.DERProgram{p}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/derp/1/derc":
			var c ieee20305.DERControl
			c.Href = new("/derp/1/derc/1")
			c.MRID = ieee20305.MRIDType{2}
			c.Interval = ieee20305.DateTimeInterval{Start: start, Duration: ^ieee20305.UInt32(0)}
			l := &ieee20305.DERControlList{DERControl: []ieee20305.DERControl{c}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/tp":
			var tp ieee20305.TariffProfile
			tp.Href = new("/tp/1")
			tp.MRID = ieee20305.MRIDType{3}
			m := ieee20305.PowerOfTenMultiplierType(127)
			tp.PricePowerOfTenMultiplier = &m
			tp.RateComponentListLink = &ieee20305.RateComponentListLink{}
			tp.RateComponentListLink.Href = "/tp/1/rc"
			l := &ieee20305.TariffProfileList{TariffProfile: []ieee20305.TariffProfile{tp}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/tp/1/rc":
			var rc ieee20305.RateComponent
			rc.Href = new("/tp/1/rc/1")
			rc.MRID = ieee20305.MRIDType{4}
			rc.TimeTariffIntervalListLink.Href = "/tp/1/rc/1/tti"
			rc.ReadingTypeLink.Href = "/rt"
			l := &ieee20305.RateComponentList{RateComponent: []ieee20305.RateComponent{rc}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/tp/1/rc/1/tti":
			var tti ieee20305.TimeTariffInterval
			tti.Href = new("/tp/1/rc/1/tti/1")
			tti.MRID = ieee20305.MRIDType{5}
			tti.Interval = ieee20305.DateTimeInterval{Start: start, Duration: ^ieee20305.UInt32(0)}
			tti.ConsumptionTariffIntervalListLink = &ieee20305.ConsumptionTariffIntervalListLink{}
			tti.ConsumptionTariffIntervalListLink.Href = "/tp/1/rc/1/tti/1/cti"
			l := &ieee20305.TimeTariffIntervalList{TimeTariffInterval: []ieee20305.TimeTariffInterval{tti}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case "/tp/1/rc/1/tti/1/cti":
			price := ieee20305.Int32(-1 << 31)
			l := &ieee20305.ConsumptionTariffIntervalList{ConsumptionTariffInterval: []ieee20305.ConsumptionTariffInterval{{ConsumptionBlock: 1, Price: &price}}}
			l.ConsumptionTariffInterval[0].Href = new("/tp/1/rc/1/tti/1/cti/1")
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		default:
			http.NotFound(w, r)
		}
	}
}

// Times a server gives that are not of years 0-9999 (which JSON times cannot
// hold), and bogus multipliers, never make a read command fail as an
// internal or output error: the answer is written, or refused as the
// server's (exit 6).
func TestHostileFarTimesAndMultipliers(t *testing.T) {
	for name, start := range map[string]ieee20305.TimeType{
		"max":      ieee20305.TimeType(1<<62 - 1),
		"min":      ieee20305.TimeType(-1 << 62),
		"year1e4":  ieee20305.TimeType(253402300800), // 10000-01-01
		"negative": -1,
	} {
		t.Run(name, func(t *testing.T) {
			newHostile(t, farTimes(t, start))
			for _, args := range [][]string{{"client", "controls"}, {"client", "programs"}, {"client", "pricing"}} {
				for _, f := range []string{"json", "text", "table"} {
					r := run(t, append(args, "--format", f)...)
					if r.code != 0 && r.code != 6 {
						t.Errorf("%s --format %s with start %d: exit %d\nstdout: %.1000s\nstderr: %.1000s",
							strings.Join(args, " "), f, start, r.code, r.stdout, r.stderr)
					}
				}
			}
		})
	}
}

// nestedLists is a server whose every list claims 100000 items and has them,
// 50 a page, each with a link to another such list: tariffs with rate
// components with intervals, programs with controls. budget is the most
// requests it answers; beyond, it answers 500.
func nestedLists(t *testing.T, budget int64) func(hs *hostile, w http.ResponseWriter, r *http.Request) {
	return func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		if hs.requests.Load() > budget {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s, _ := strconv.Atoi(r.URL.Query().Get("s"))
		const all, page = 100000, 50
		p := r.URL.Path
		href := func(i int) string { return p + "/" + strconv.Itoa(s+i) }
		switch {
		case p == "/fsa":
			var f ieee20305.FunctionSetAssignments
			f.Href = new("/fsa/1")
			f.DERProgramListLink = &ieee20305.DERProgramListLink{}
			f.DERProgramListLink.Href = "/derp"
			f.TariffProfileListLink = &ieee20305.TariffProfileListLink{}
			f.TariffProfileListLink.Href = "/tp"
			l := &ieee20305.FunctionSetAssignmentsList{FunctionSetAssignments: []ieee20305.FunctionSetAssignments{f}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		case p == "/derp":
			l := &ieee20305.DERProgramList{}
			for i := range page {
				var x ieee20305.DERProgram
				x.Href = new(href(i))
				x.MRID = ieee20305.MRIDType{1}
				x.DERControlListLink = &ieee20305.DERControlListLink{}
				x.DERControlListLink.Href = href(i) + "/derc"
				l.DERProgram = append(l.DERProgram, x)
			}
			l.All, l.Results = all, page
			writeDoc(t, w, l)
		case strings.HasSuffix(p, "/derc"):
			l := &ieee20305.DERControlList{}
			for i := range page {
				var x ieee20305.DERControl
				x.Href = new(href(i))
				x.MRID = ieee20305.MRIDType{2}
				l.DERControl = append(l.DERControl, x)
			}
			l.All, l.Results = all, page
			writeDoc(t, w, l)
		case p == "/tp":
			l := &ieee20305.TariffProfileList{}
			for i := range page {
				var x ieee20305.TariffProfile
				x.Href = new(href(i))
				x.MRID = ieee20305.MRIDType{3}
				x.RateComponentListLink = &ieee20305.RateComponentListLink{}
				x.RateComponentListLink.Href = href(i) + "/rc"
				l.TariffProfile = append(l.TariffProfile, x)
			}
			l.All, l.Results = all, page
			writeDoc(t, w, l)
		case strings.HasSuffix(p, "/rc"):
			l := &ieee20305.RateComponentList{}
			for i := range page {
				var x ieee20305.RateComponent
				x.Href = new(href(i))
				x.MRID = ieee20305.MRIDType{4}
				x.TimeTariffIntervalListLink.Href = href(i) + "/tti"
				x.ReadingTypeLink.Href = "/rt"
				l.RateComponent = append(l.RateComponent, x)
			}
			l.All, l.Results = all, page
			writeDoc(t, w, l)
		case strings.HasSuffix(p, "/tti"):
			l := &ieee20305.TimeTariffIntervalList{}
			for i := range page {
				var x ieee20305.TimeTariffInterval
				x.Href = new(href(i))
				x.MRID = ieee20305.MRIDType{5}
				l.TimeTariffInterval = append(l.TimeTariffInterval, x)
			}
			l.All, l.Results = all, page
			writeDoc(t, w, l)
		default:
			http.NotFound(w, r)
		}
	}
}

// Lists within lists are bounded as a whole, not each by itself: a server
// whose every list is as long as the library allows cannot keep a one-shot
// command going for 100000^n requests.
func TestHostileNestedListsBounded(t *testing.T) {
	skipDefect(t, "client pricing (cmd/client/report.go pricing/rateComponent) and client controls/programs "+
		"(cmd/client/der.go derPrograms/readProgram) bound each list (100000 items) but not the product of nested "+
		"lists: a hostile server makes them run for 100000^2..^3 requests and hold every item in memory")
	const budget = 6000 // three lists of the library's 100000 items in pages of 50
	for _, cmd := range []string{"pricing", "controls", "programs"} {
		t.Run(cmd, func(t *testing.T) {
			hs := newHostile(t, nestedLists(t, budget))
			r := run(t, "client", cmd, "--format", "json")
			if n := hs.requests.Load(); n > budget {
				t.Fatalf("client %s made %d requests (exit %d): the server stopped it, not the client", cmd, n, r.code)
			}
			if r.code != 0 && r.code != 6 {
				t.Fatalf("client %s: exit %d\n%.1000s", cmd, r.code, r.stdout)
			}
		})
	}
}

// A server that fails the agent's start (a program list that answers 503,
// or a server down when the command starts) is a failure client run
// reports while it retries: a record, not silence.
func TestHostileRunStartFailureIsReported(t *testing.T) {
	skipDefect(t, "client run (cmd/client/run.go) writes no record while the agent's Start fails and is retried "+
		"with backoff (Metrics.OnPoll is only called once started, and the agent's log is discarded): against a "+
		"server that answers 503, or is down at launch, the stream is empty until --duration ends, exit 0")
	newHostile(t, func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		switch r.URL.Path {
		case "/fsa":
			var f ieee20305.FunctionSetAssignments
			f.Href = new("/fsa/1")
			f.DERProgramListLink = &ieee20305.DERProgramListLink{}
			f.DERProgramListLink.Href = "/derp"
			l := &ieee20305.FunctionSetAssignmentsList{FunctionSetAssignments: []ieee20305.FunctionSetAssignments{f}}
			l.All, l.Results = 1, 1
			writeDoc(t, w, l)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	r := run(t, "client", "run", "--duration", "3s", "--poll-rate", "1s", "--format", "json")
	if r.code != 0 {
		t.Fatalf("client run: exit %d\n%s", r.code, r.stdout)
	}
	evs := events(t, r.stdout)
	failed := has(evs, func(e event) bool { return strings.HasSuffix(e.Event, "_failed") })
	if !failed {
		t.Fatalf("no failure record while every start failed:\n%s", r.stdout)
	}
}

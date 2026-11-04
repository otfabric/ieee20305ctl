// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	ieee20305 "github.com/otfabric/go-ieee20305"
)

// A malformed environment variable is invalid input (exit 3), and with
// --format json (or IEEE20305CTL_FORMAT=json) the error is the JSON object
// on stdout that docs/exitcodes.md promises, not a line on stderr.
func TestInvalidEnvironmentIsJSONWithFormatJSON(t *testing.T) {
	for _, env := range []string{"IEEE20305CTL_TIMEOUT=abc", "IEEE20305CTL_TIMEOUT=9999999999h", "IEEE20305CTL_PIN=-1",
		"IEEE20305CTL_PIN=4294967296", "IEEE20305CTL_CCM8=maybe"} {
		name, value, _ := strings.Cut(env, "=")
		t.Run(env, func(t *testing.T) {
			t.Setenv(name, value)
			for _, args := range [][]string{
				{"client", "dcap", "--format", "json"},
				{"client", "dcap", "--format=json"},
				{"--debug", "cert", "lfdi", "x.pem", "--format", "json"},
			} {
				r := run(t, args...)
				if r.code != 3 {
					t.Fatalf("%s: exit %d, want 3", strings.Join(args, " "), r.code)
				}
				if got := errorCode(t, r); got != "invalid_input" {
					t.Fatalf("%s: code %q", strings.Join(args, " "), got)
				}
			}
			t.Setenv("IEEE20305CTL_FORMAT", "json")
			if got := errorCode(t, run(t, "client", "dcap")); got != "invalid_input" {
				t.Fatalf("IEEE20305CTL_FORMAT=json: code %q", got)
			}
			t.Setenv("IEEE20305CTL_FORMAT", "")
			if r := run(t, "client", "dcap"); r.code != 3 || r.stdout != "" || !strings.Contains(r.stderr, name) {
				t.Fatalf("text: %+v", r)
			}
		})
	}
}

// Numeric flags out of their type's range, or not numbers, are usage
// errors (exit 2); durations and values that fit the type but not the
// protocol are invalid input (exit 3) and nothing is sent.
func TestNumericFlagBoundaries(t *testing.T) {
	t.Setenv("IEEE20305CTL_URL", "127.0.0.1:1")
	t.Setenv("IEEE20305CTL_CERT", "x.pem")
	t.Setenv("IEEE20305CTL_KEY", "x.key")
	t.Setenv("IEEE20305CTL_CA", "ca.pem")
	for _, c := range []struct {
		code int
		args []string
	}{
		{2, []string{"client", "register", "--pin", "-1"}},
		{2, []string{"client", "register", "--pin", "4294967296"}},
		{2, []string{"client", "register", "--pin", "NaN"}},
		{2, []string{"client", "report", "--power", "9223372036854775808"}},
		{2, []string{"client", "report", "--power", "1e3"}},
		{2, []string{"client", "log-event", "--code", "256"}},
		{2, []string{"client", "log-event", "--id", "65536"}},
		{2, []string{"client", "subscribe", "/x", "--limit", "-1"}},
		{2, []string{"client", "subscribe", "/x", "--count", "99999999999999999999"}},
		{2, []string{"client", "flow-reservation", "--duration", "9999999999h"}},
		{2, []string{"client", "flow-reservation", "--duration", "NaN"}},
		{3, []string{"client", "flow-reservation", "--duration", "-1h"}},
		{3, []string{"client", "flow-reservation", "--duration", "0s"}},
		{3, []string{"client", "flow-reservation", "--start", "-1s"}},
		{3, []string{"client", "dcap", "--timeout", "0s"}},
		{3, []string{"client", "dcap", "--timeout", "-1ns"}},
		{3, []string{"client", "log-event", "--details", strings.Repeat("x", 33)}},
		{3, []string{"client", "discover", "--wait", "-1s"}},
		{3, []string{"client", "discover", "--wait", "0s"}},
	} {
		r := run(t, append(c.args, "--format", "json")...)
		if r.code == 3 && c.code == 3 {
			// nothing was sent: no connection to 127.0.0.1:1 was tried
			if strings.Contains(r.stdout, "connect") {
				t.Errorf("%s: %s", strings.Join(c.args, " "), r.stdout)
			}
		}
		if r.code != c.code {
			t.Errorf("%s: exit %d, want %d\n%s%s", strings.Join(c.args, " "), r.code, c.code, r.stdout, r.stderr)
		}
	}
}

// Extremes of --energy, --power and --duration are scaled into the
// protocol's types, or refused before anything is posted; never wrapped.
func TestFlowReservationExtremes(t *testing.T) {
	var mu sync.Mutex
	var posted []*ieee20305.FlowReservationRequest
	newHostile(t, func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/frq" {
			http.NotFound(w, r)
			return
		}
		data, _ := io.ReadAll(r.Body)
		var req ieee20305.FlowReservationRequest
		if err := ieee20305.Unmarshal(data, &req); err != nil {
			t.Errorf("the posted request does not decode: %v\n%s", err, data)
		}
		mu.Lock()
		posted = append(posted, &req)
		mu.Unlock()
		w.Header().Set("Location", "/frq/1")
		w.WriteHeader(http.StatusCreated)
	})
	last := func() *ieee20305.FlowReservationRequest {
		mu.Lock()
		defer mu.Unlock()
		if len(posted) == 0 {
			t.Fatal("nothing posted")
		}
		return posted[len(posted)-1]
	}
	for _, e := range []int64{math.MaxInt64, math.MinInt64, 1 << 47, -(1 << 47) - 1, 0} {
		mustCode(t, 0, "", "client", "flow-reservation", "--energy", strconv.FormatInt(e, 10), "--power", "1000")
		got := last().EnergyRequested
		v := float64(got.Value) * math.Pow10(int(got.Multiplier))
		if math.Abs(v-float64(e)) > math.Abs(float64(e))*1e-12 || int64(got.Value) > 1<<47-1 || int64(got.Value) < -(1<<47) {
			t.Errorf("--energy %d posted %d x 10^%d", e, got.Value, got.Multiplier)
		}
	}
	for _, p := range []int64{32767, -32768, 32768 * 10, -3276800000, 1 << 62} {
		r := run(t, "client", "flow-reservation", "--energy", "1", "--power", strconv.FormatInt(p, 10), "--format", "json")
		if r.code == 3 {
			continue // refused: digits it cannot keep
		}
		if r.code != 0 {
			t.Fatalf("--power %d: exit %d\n%s", p, r.code, r.stdout)
		}
		got := last().PowerRequested
		if v := int64(got.Value) * int64(math.Pow10(int(got.Multiplier))); v != p {
			t.Errorf("--power %d posted %d x 10^%d", p, got.Value, got.Multiplier)
		}
	}
	mustCode(t, 3, "invalid_input", "client", "flow-reservation", "--energy", "1", "--power", "32769")
	// The longest interval saturates at the UInt32 of seconds.
	mustCode(t, 0, "", "client", "flow-reservation", "--energy", "1", "--power", "1", "--duration", "2562047h")
	if d := last().IntervalRequested.Duration; d < 1<<31 {
		t.Errorf("--duration 2562047h posted a duration of %d s", d)
	}
	mustCode(t, 0, "", "client", "flow-reservation", "--energy", "1", "--power", "1", "--start", "2562047h", "--duration", "1h")
	mustCode(t, 3, "", "client", "flow-reservation", "--energy", "1", "--power", "1", "--duration-requested", "65536s")
}

// A --duration under a second is positive, so accepted; it is not posted as
// an interval of no time.
func TestFlowReservationSubSecondDuration(t *testing.T) {
	skipDefect(t, "client flow-reservation (cmd/client/report.go) accepts --duration 500ms (positive) and posts "+
		"an IntervalRequested of duration 0; refuse it (exit 3) or round up")
	var dur atomic.Int64
	dur.Store(-1)
	newHostile(t, func(hs *hostile, w http.ResponseWriter, r *http.Request) {
		if hs.serveBase(t, w, r, nil) {
			return
		}
		data, _ := io.ReadAll(r.Body)
		var req ieee20305.FlowReservationRequest
		if ieee20305.Unmarshal(data, &req) == nil {
			dur.Store(int64(req.IntervalRequested.Duration))
		}
		w.Header().Set("Location", "/frq/1")
		w.WriteHeader(http.StatusCreated)
	})
	r := run(t, "client", "flow-reservation", "--energy", "1", "--power", "1", "--duration", "500ms", "--format", "json")
	if r.code == 0 && dur.Load() == 0 {
		t.Fatal("--duration 500ms posted an interval of 0 s")
	}
}

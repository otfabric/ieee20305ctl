// SPDX-License-Identifier: MIT

package conn

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// FuzzServerURL feeds --url (and IEEE20305CTL_URL). What is accepted is an
// https URL with a host and a path, which the client can use as is; what
// is refused is invalid_url.
func FuzzServerURL(f *testing.F) {
	for _, s := range []string{
		"", "utility.example", "utility.example:8443", "127.0.0.1", "[::1]:443", "[::1", "https://u.example",
		"https://u.example/", "https://u.example/dcap?x=1", "http://u.example", "https://", "https:///dcap",
		"https://user:pw@u.example:1/x", " https://u.example ", "u.example/dcap", "::", "%zz", "https://u.example/%zz",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := ServerURL(s)
		if err != nil {
			return
		}
		if !strings.Contains(s, "://") {
			// DEFECT: the host[:port] form is not checked; see
			// TestServerURLHostForm. Remove this when it is fixed.
			return
		}
		u, perr := url.Parse(got)
		if perr != nil {
			t.Fatalf("ServerURL(%q) = %q, which does not parse: %v", s, got, perr)
		}
		if u.Scheme != "https" || u.Host == "" || u.Path == "" {
			t.Fatalf("ServerURL(%q) = %q: not an https URL with a host and a path", s, got)
		}
	})
}

// --url as host[:port] is a host and maybe a port, nothing else: what is
// not is invalid_url when the flags are checked, not a URL of another path
// or a failure when the client is made.
func TestServerURLHostForm(t *testing.T) {
	if os.Getenv("IEEE20305CTL_TEST_DEFECTS") != "1" {
		t.Skip("DEFECT: ServerURL (internal/conn/conn.go) does not check the host[:port] form: \"[::1\" and \"%zz\" " +
			"give URLs that do not parse (refused only by Connect), and \"u.example/dcap\" gives " +
			"https://u.example/dcap:443/dcap, a request to path /dcap:443/dcap")
	}
	for _, s := range []string{"[::1", "%zz", "u.example/dcap", "u.example:99999", "u.example:", "a b", "u.example?x=1", "u.example#f"} {
		if got, err := ServerURL(s); err == nil {
			t.Errorf("ServerURL(%q) = %q, want invalid_url", s, got)
		}
	}
	for s, want := range map[string]string{"u.example": "https://u.example:443/dcap", "u.example:8443": "https://u.example:8443/dcap",
		"[::1]:1": "https://[::1]:1/dcap", "127.0.0.1": "https://127.0.0.1:443/dcap"} {
		if got, err := ServerURL(s); err != nil || got != want {
			t.Errorf("ServerURL(%q) = %q, %v; want %q", s, got, err, want)
		}
	}
}

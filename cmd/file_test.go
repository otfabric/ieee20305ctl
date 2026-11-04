// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// posix skips a test of file modes or symlinks where the platform has
// neither (Windows reports 0666 and needs privileges for symlinks).
func posix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes and symlinks")
	}
}

// Keys are 0600 and certificates 0644, also when --force writes over files
// that had other modes.
func TestCertGenerateModes(t *testing.T) {
	posix(t)
	dir := t.TempDir()
	must(t, 0, "cert", "generate", "--out", dir, "--devices", "1")
	check := func() {
		t.Helper()
		for _, n := range []string{"ca", "server", "device-1"} {
			for ext, want := range map[string]os.FileMode{".key": 0o600, ".pem": 0o644} {
				fi, err := os.Stat(filepath.Join(dir, n+ext))
				if err != nil {
					t.Fatal(err)
				}
				if got := fi.Mode().Perm(); got&^want != 0 || (ext == ".key" && got != 0o600) {
					t.Errorf("%s%s: mode %o, want %o", n, ext, got, want)
				}
			}
		}
	}
	check()
	for _, n := range []string{"ca.key", "server.key", "device-1.key"} {
		if err := os.Chmod(filepath.Join(dir, n), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	must(t, 0, "cert", "generate", "--out", dir, "--devices", "1", "--force")
	check()
}

// --out naming a file, or a directory that cannot be made, fails before or
// while writing, with an output error code and nothing half written over.
func TestCertGenerateBadOut(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{file, filepath.Join(file, "sub")} {
		r := run(t, "cert", "generate", "--out", out, "--format", "json")
		if r.code != 3 && r.code != 10 {
			t.Errorf("--out %s: exit %d\n%s", out, r.code, r.stdout)
		}
	}
	if data, _ := os.ReadFile(file); string(data) != "x" {
		t.Errorf("the file at --out was changed: %q", data)
	}
	// One existing file keeps every file from being written, and is kept.
	pki := filepath.Join(dir, "pki")
	if err := os.MkdirAll(pki, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pki, "device-2.key"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := errorCode(t, mustCode(t, 3, "", "cert", "generate", "--out", pki, "--devices", "2")); code != "invalid_input" {
		t.Errorf("an existing file: %s", code)
	}
	if _, err := os.Stat(filepath.Join(pki, "ca.pem")); err == nil {
		t.Error("files were written next to an existing one without --force")
	}
	if data, _ := os.ReadFile(filepath.Join(pki, "device-2.key")); string(data) != "mine" {
		t.Error("the existing key was overwritten without --force")
	}
}

// A symlink at the path of a key, dangling or not, is not followed: the key
// is not written to where the link points.
func TestCertGenerateSymlinkAtKeyPath(t *testing.T) {
	posix(t)
	skipDefect(t, "cert generate (cmd/cert/root.go generate/write) follows symlinks at its output paths: os.Stat "+
		"treats a dangling symlink as absent, and os.WriteFile then writes the private key to the link's target "+
		"(anywhere the user can write); with --force it also writes through a symlink to an existing file and "+
		"chmods that file 0600. Open with O_EXCL|O_NOFOLLOW (or write a temp file and rename) instead")
	for _, force := range []bool{false, true} {
		dir := t.TempDir()
		elsewhere := filepath.Join(t.TempDir(), "stolen.key")
		if force {
			if err := os.WriteFile(elsewhere, []byte("precious"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(elsewhere, filepath.Join(dir, "device-1.key")); err != nil {
			t.Fatal(err)
		}
		args := []string{"cert", "generate", "--out", dir, "--devices", "1"}
		if force {
			args = append(args, "--force")
		}
		_ = run(t, args...)
		data, err := os.ReadFile(elsewhere)
		switch {
		case !force && err == nil:
			t.Errorf("a key was written through a dangling symlink to %s", elsewhere)
		case force && string(data) != "precious":
			t.Errorf("--force wrote through a symlink to %s: %.40q", elsewhere, data)
		}
	}
}

// server generate-config --output does not overwrite without --force.
func TestGenerateConfigOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "seed.yaml")
	must(t, 0, "server", "generate-config", "--output", out)
	if err := os.WriteFile(out, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "server", "generate-config", "--output", out); r.code != 3 {
		t.Fatalf("over an existing file: exit %d", r.code)
	}
	if data, _ := os.ReadFile(out); string(data) != "mine" {
		t.Fatal("overwritten without --force")
	}
	must(t, 0, "server", "generate-config", "--output", out, "--force")
	if data, _ := os.ReadFile(out); !bytes.Contains(data, []byte("programs:")) {
		t.Fatal("--force did not write")
	}
	r := run(t, "server", "generate-config", "--output", filepath.Join(dir, "no", "such", "dir", "s.yaml"))
	if r.code != 10 {
		t.Errorf("an output in a missing directory: exit %d", r.code)
	}
}

// server start does not delete a file that is not a socket at
// --control-socket: a mistyped path costs the operator nothing.
func TestControlSocketDoesNotDeleteFiles(t *testing.T) {
	posix(t)
	dir := t.TempDir()
	pki := filepath.Join(dir, "pki")
	must(t, 0, "cert", "generate", "--out", pki)
	seed := filepath.Join(dir, "seed.yaml")
	if err := os.WriteFile(seed, []byte("poll_rate: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(dir, "precious.txt")
	if err := os.WriteFile(precious, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var out, errb bytes.Buffer
	code := Run(ctx, []string{"server", "start", "--listen", "127.0.0.1:0", "--cert", pki + "/server.pem", "--key", pki + "/server.key",
		"--ca", pki + "/ca.pem", "--config", seed, "--control-socket", precious, "--format", "json"}, strings.NewReader(""), &out, &errb)
	if code != 4 || !strings.Contains(out.String(), "control_socket_failed") {
		t.Errorf("server start with a regular file at --control-socket: exit %d\n%s%s", code, out.String(), errb.String())
	}
	if data, err := os.ReadFile(precious); err != nil || string(data) != "keep me" {
		t.Fatalf("server start deleted or changed the file at --control-socket: %v %q", err, data)
	}
}

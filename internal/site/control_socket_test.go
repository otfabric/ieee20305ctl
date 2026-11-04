// SPDX-License-Identifier: MIT

package site

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ServeControl replaces a socket a server left behind, and refuses (and
// keeps) anything else at its path.
func TestServeControlPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets as files")
	}
	dir, err := os.MkdirTemp("", "i2s") // a socket path must be short
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Site{}

	stale := filepath.Join(dir, "stale.sock")
	l, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
	if err := s.ServeControl(ctx, stale); err != nil {
		t.Fatalf("a stale socket was not replaced: %v", err)
	}

	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.ServeControl(ctx, file); err == nil {
		t.Error("a regular file at the path was replaced")
	}
	if data, _ := os.ReadFile(file); string(data) != "keep" {
		t.Errorf("the file was changed: %q", data)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.ServeControl(ctx, sub); err == nil {
		t.Error("a directory at the path was replaced")
	}
	if _, err := os.Stat(sub); err != nil {
		t.Errorf("the directory: %v", err)
	}
}

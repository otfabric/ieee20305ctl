// SPDX-License-Identifier: MIT

//go:build unix

package site

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The control socket has no permissions for the group and others from the
// moment it exists, whatever the umask of the process, and the umask is
// left as it was.
func TestControlSocketCreatedPrivate(t *testing.T) {
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	dir, err := os.MkdirTemp("", "i2p") // a socket path must be short
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "c.sock")
	l, err := listenUnix(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("the socket was created with mode %v", perm)
	}
	if now := syscall.Umask(0); now != 0 {
		t.Errorf("the umask after listenUnix is %#o, want 0", now)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := filepath.Join(dir, "s.sock")
	if err := (&Site{}).ServeControl(ctx, served); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(served); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("the served socket: %v, %v", fi.Mode(), err)
	}
}

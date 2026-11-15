// SPDX-License-Identifier: MIT

//go:build unix

package site

import (
	"net"
	"sync"
	"syscall"
)

// umaskMu serializes the umask changes of listenUnix.
var umaskMu sync.Mutex

// listenUnix listens on a Unix socket at path that has no permissions for
// the group and others from the moment it exists: the umask is 0077 while
// the socket is created. The umask is the process's, so a file another
// goroutine creates at that moment gets no group or other permissions
// either.
func listenUnix(path string) (net.Listener, error) {
	umaskMu.Lock()
	defer umaskMu.Unlock()
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	return net.Listen("unix", path)
}

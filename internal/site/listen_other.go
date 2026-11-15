// SPDX-License-Identifier: MIT

//go:build !unix

package site

import "net"

// listenUnix listens on a Unix socket at path. Outside Unix there is no
// umask; the permissions of the socket are those its directory gives.
func listenUnix(path string) (net.Listener, error) { return net.Listen("unix", path) }

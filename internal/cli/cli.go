// SPDX-License-Identifier: MIT

// Package cli provides common CLI helpers for ieee20305ctl.
package cli

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
)

// LoadTLSCertificate loads a TLS certificate and private key from the given paths.
func LoadTLSCertificate(certFile, keyFile string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("loading certificate: %w", err)
	}
	return cert, nil
}

// LoadCACertPool loads a CA certificate file and returns a certificate pool.
func LoadCACertPool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no valid certificates found in %s", caFile)
	}
	return pool, nil
}

// ParseAddress normalises a server address, adding a default port if missing.
func ParseAddress(addr, defaultPort string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ":" + defaultPort
	}
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port specified; append default.
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

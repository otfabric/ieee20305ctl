// SPDX-License-Identifier: MIT

// Package errs defines typed exit codes for ieee20305ctl.
package errs

// Exit codes used by ieee20305ctl.
const (
	// ExitOK indicates successful execution.
	ExitOK = 0

	// ExitError indicates a general runtime error.
	ExitError = 1

	// ExitUsage indicates incorrect command usage or invalid arguments.
	ExitUsage = 2

	// ExitConnect indicates a connection or TLS handshake failure.
	ExitConnect = 3

	// ExitCertificate indicates a certificate-related error (missing, invalid, expired).
	ExitCertificate = 4

	// ExitInvalid indicates that a document does not satisfy the IEEE 2030.5
	// schema.
	ExitInvalid = 5
)

// ExitCodeError wraps an error with a specific exit code.
type ExitCodeError struct {
	Code int
	Err  error
}

// Error returns the underlying error message.
func (e *ExitCodeError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *ExitCodeError) Unwrap() error {
	return e.Err
}

// WithCode wraps an error with the given exit code.
func WithCode(code int, err error) *ExitCodeError {
	return &ExitCodeError{Code: code, Err: err}
}

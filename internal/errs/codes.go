// SPDX-License-Identifier: MIT

package errs

// Stable machine-oriented codes for JSON error.code and scripts (docs/exitcodes.md).
const (
	CodeInvalidFormat          = "invalid_format"
	CodeInvalidInput           = "invalid_input"
	CodeInvalidURL             = "invalid_url"
	CodeInvalidFlagCombination = "invalid_flag_combination"
	CodeInvalidConfig          = "invalid_config"
	CodeInvalidDocument        = "invalid_document"
	CodeInvalidCertificate     = "invalid_certificate"
	CodeUnknownResource        = "unknown_resource"
	// A command that changes a running server was not confirmed with --yes.
	CodeConfirmationRequired = "confirmation_required"

	CodeTransportConnectFailed = "transport_connect_failed"
	// The TLS handshake failed: a certificate either end refused, or no common suite.
	CodeTransportTLSFailed  = "transport_tls_failed"
	CodeConnectionLost      = "connection_lost"
	CodeListenFailed        = "listen_failed"
	CodeControlSocketFailed = "control_socket_failed"

	// No answer in time to a request that does not change state.
	CodeRequestTimeout = "request_timeout"
	// No answer to a request that may have changed state: it is not resent.
	CodeOutcomeUnknown = "outcome_unknown"

	// The server answered with an HTTP error: the ieee20305 object has the status
	// and the server's Error document.
	CodeNotFound    = "not_found"
	CodeForbidden   = "forbidden"
	CodeBadRequest  = "bad_request"
	CodeConflict    = "conflict"
	CodeServerError = "server_error"
	CodeUnavailable = "unavailable"
	CodeHTTPError   = "http_error"
	// The server's answer breaks the schema or the protocol.
	CodeInvalidResponse   = "invalid_response"
	CodeProtocolViolation = "protocol_violation"
	// The server is not the one the device was given: its Registration has another PIN.
	CodePINMismatch = "pin_mismatch"
	// The server has not registered the device.
	CodeNotRegistered = "not_registered"

	CodeContextCanceled = "context_canceled"

	CodeJSONEncodeFailed       = "json_encode_failed"
	CodeTableRenderFailed      = "table_render_failed"
	CodeOutputWriteFailed      = "output_write_failed"
	CodeOutputFileWriteFailed  = "output_file_write_failed"
	CodeOutputFileCreateFailed = "output_file_create_failed"
	CodeInputFileOpenFailed    = "input_file_open_failed"
	CodeInputDecodeFailed      = "input_decode_failed"

	CodeInternalUnexpected = "internal_unexpected"
)

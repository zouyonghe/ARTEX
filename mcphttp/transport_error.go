package mcphttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
)

// HTTP transport errors can embed credentials in a request or Location URL.
// Retain classification via Unwrap, but never format those URLs for logs/UI.
type mcpTransportError struct{ cause error }

func (e mcpTransportError) Error() string {
	category := "request failure"
	var dnsErr *net.DNSError
	var tlsErr *tls.CertificateVerificationError
	var tlsRecordErr tls.RecordHeaderError
	var opErr *net.OpError
	var netErr net.Error
	var escapeErr url.EscapeError
	var hostErr url.InvalidHostError
	switch {
	case errors.Is(e.cause, context.Canceled):
		category = "canceled"
	case errors.Is(e.cause, context.DeadlineExceeded), errors.As(e.cause, &netErr) && netErr.Timeout():
		category = "timeout"
	case errors.As(e.cause, &dnsErr):
		category = "DNS failure"
	case errors.As(e.cause, &tlsErr), errors.As(e.cause, &tlsRecordErr):
		category = "TLS failure"
	case errors.As(e.cause, &escapeErr), errors.As(e.cause, &hostErr):
		category = "invalid URL"
	case errors.As(e.cause, &opErr):
		category = "connection failure"
	}
	return "mcp HTTP " + category + " (URL details hidden)"
}
func (e mcpTransportError) Unwrap() error { return e.cause }

func safeMCPTransportError(err error) error {
	if errors.Is(err, errMCPRedirect) {
		return errMCPRedirect
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return mcpTransportError{cause: err}
	}
	return err
}

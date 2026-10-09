package mcphttp

import (
	"errors"
	"net/url"
)

// HTTP transport errors can embed credentials in a request or Location URL.
// Retain classification via Unwrap, but never format those URLs for logs/UI.
type mcpTransportError struct{ cause error }

func (e mcpTransportError) Error() string { return "mcp HTTP transport failed (URL details hidden)" }
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

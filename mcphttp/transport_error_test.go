package mcphttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestMCPTransportErrorPreservesCauseWithoutURL(t *testing.T) {
	original := &url.Error{Op: "Post", URL: "https://fixture.invalid/?token=fixture-secret", Err: context.DeadlineExceeded}
	got := safeMCPTransportError(original)
	var typed *url.Error
	if strings.Contains(got.Error(), "fixture-secret") || !errors.Is(got, context.DeadlineExceeded) || !errors.As(got, &typed) {
		t.Fatal("transport error leaked URL or lost classification")
	}
	if safeMCPTransportError(&url.Error{Op: "Get", URL: "fixture-secret", Err: errMCPRedirect}) != errMCPRedirect {
		t.Fatal("redirect rejection must not retain URL-bearing wrapper")
	}
	other := errors.New("fixture cause")
	if safeMCPTransportError(other) != other {
		t.Fatal("unrelated error changed")
	}
}

func TestMalformedMCPRequestURLDoesNotEchoCredentials(t *testing.T) {
	const target = "http://fixture.invalid/%zz?token=request-fixture-secret"
	for _, constructor := range []func(context.Context, string, string, map[string]string, bool) (*Client, error){New, NewSSE} {
		client, err := constructor(context.Background(), "fixture", target, nil, false)
		if client != nil {
			client.Close()
		}
		if err == nil || strings.Contains(err.Error(), "request-fixture-secret") {
			t.Fatalf("request URL was not safely rejected: %v", err)
		}
	}
	client := &Client{messageURL: target}
	_, err := client.legacyRoundTrip(context.Background(), rpcRequest{Method: "notifications/initialized"}, false)
	if err == nil || strings.Contains(err.Error(), "request-fixture-secret") {
		t.Fatalf("legacy POST request URL was not safely rejected: %v", err)
	}
}

func TestMCPTransportErrorReportsSafeCategory(t *testing.T) {
	for _, tc := range []struct {
		cause error
		want  string
	}{
		{&net.DNSError{Err: "fixture-secret", Name: "fixture-secret"}, "DNS"},
		{&tls.CertificateVerificationError{Err: errors.New("fixture-secret")}, "TLS"},
		{&net.OpError{Op: "dial", Err: errors.New("fixture-secret")}, "connection"},
		{context.DeadlineExceeded, "timeout"},
		{context.Canceled, "canceled"},
		{url.EscapeError("fixture-secret"), "invalid URL"},
	} {
		got := safeMCPTransportError(&url.Error{Op: "Post", URL: "fixture-secret", Err: tc.cause})
		if !strings.Contains(got.Error(), tc.want) || strings.Contains(got.Error(), "fixture-secret") {
			t.Fatalf("safe error category missing or secret exposed: %v", got)
		}
		if !errors.Is(got, tc.cause) {
			t.Fatal("underlying cause lost")
		}
	}
}

package mcphttp

import (
	"context"
	"errors"
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

package mcphttp

import (
	"bufio"
	"strings"
	"testing"
)

func TestSSEEndpointRejectsDifferentOrigin(t *testing.T) {
	for _, candidate := range []string{
		"https://attacker.invalid/message",
		"//attacker.invalid/message",
		"file:///tmp/message",
		"http://trusted.example/message",
		"https://trusted.example:8443/message",
		"https://user:fixture@trusted.example/message",
	} {
		if got, err := readSSEEndpoint(bufio.NewReader(strings.NewReader("data: "+candidate+"\n\n")), "https://trusted.example/sse"); err == nil || got != "" {
			t.Fatalf("endpoint %q should be rejected, got %q err=%v", candidate, got, err)
		}
	}
}

func TestSSEEndpointAcceptsSameOrigin(t *testing.T) {
	for _, candidate := range []string{
		"/message?sessionId=fixture",
		"message?sessionId=fixture",
		"https://trusted.example/message",
		"https://TRUSTED.example:443/message",
	} {
		if _, err := readSSEEndpoint(bufio.NewReader(strings.NewReader("data: "+candidate+"\n\n")), "https://trusted.example/sse"); err != nil {
			t.Fatalf("same-origin endpoint rejected: %v", err)
		}
	}
}

func TestSSEEndpointParseErrorDoesNotEchoURL(t *testing.T) {
	const secret = "SSE-ENDPOINT-PROBE"
	got, err := readSSEEndpoint(
		bufio.NewReader(strings.NewReader("data: https://trusted.example:bad/message?token="+secret+"\n\n")),
		"https://trusted.example/sse",
	)
	if err == nil || got != "" || strings.Contains(err.Error(), secret) {
		t.Fatalf("invalid endpoint leaked or was accepted: got=%q err=%v", got, err)
	}
}

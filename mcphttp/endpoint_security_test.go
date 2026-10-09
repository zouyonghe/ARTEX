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
	for _, tc := range []struct{ candidate, want string }{
		{"/message?sessionId=fixture", "https://trusted.example/message?sessionId=fixture"},
		{"message?sessionId=fixture", "https://trusted.example/message?sessionId=fixture"},
		{"https://trusted.example/message", "https://trusted.example/message"},
		{"https://TRUSTED.example:443/message", "https://TRUSTED.example:443/message"},
	} {
		if got, err := readSSEEndpoint(bufio.NewReader(strings.NewReader("data: "+tc.candidate+"\n\n")), "https://trusted.example/sse"); err != nil || got != tc.want {
			t.Fatalf("got=%q want=%q err=%v", got, tc.want, err)
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

func TestSSEEndpointOriginEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name, base, candidate, want string
	}{
		{"numeric port", "https://trusted.example/sse", "https://trusted.example:0443/message", "https://trusted.example:0443/message"},
		{"IPv6", "http://[::1]/sse", "http://[0:0:0:0:0:0:0:1]/message", "http://[0:0:0:0:0:0:0:1]/message"},
		{"relative credentials", "https://user:fixture@trusted.example/sse", "/message", "https://user:fixture@trusted.example/message"},
		{"uppercase scheme", "https://trusted.example/sse", "HTTPS://trusted.example/message", "https://trusted.example/message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readSSEEndpoint(bufio.NewReader(strings.NewReader("data: "+tc.candidate+"\n\n")), tc.base)
			if err != nil || got != tc.want {
				t.Fatalf("got=%q want=%q err=%v", got, tc.want, err)
			}
		})
	}
	got, err := readSSEEndpoint(bufio.NewReader(strings.NewReader("data: http://[fe80::1%25ETH0]/message\n\n")), "http://[fe80::1%25eth0]/sse")
	if err == nil || got != "" {
		t.Fatal("case-distinct IPv6 interface zones must not be considered the same origin")
	}
}

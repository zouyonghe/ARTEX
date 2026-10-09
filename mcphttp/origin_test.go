package mcphttp

import (
	"net/url"
	"testing"
)

func TestMCPOriginIDNAAndDistinctLiteralHosts(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"https://bücher.example/rpc", "https://xn--bcher-kva.example/rpc", true},
		{"https://bücher.example/rpc", "https://other.example/rpc", false},
		{"http://127.0.0.1/rpc", "http://[::ffff:127.0.0.1]/rpc", false},
	} {
		a, err := url.Parse(tc.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := url.Parse(tc.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := sameMCPOrigin(a, b); got != tc.want {
			t.Fatalf("same origin for %q and %q: got=%v want=%v", tc.a, tc.b, got, tc.want)
		}
	}
}

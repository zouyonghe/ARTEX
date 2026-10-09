package mcphttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPRedirectDoesNotForwardCustomCredentials(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
			t.Run(http.StatusText(status)+map[bool]string{true: " SSE", false: " HTTP"}[legacy], func(t *testing.T) {
				var received atomic.Bool
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("X-API-Key") == "redirect-fixture-secret" {
						received.Store(true)
					}
					w.WriteHeader(http.StatusBadRequest)
				}))
				defer target.Close()
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, target.URL+"/message?token=redirect-fixture-secret", status)
				}))
				defer origin.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				constructor := New
				if legacy {
					constructor = NewSSE
				}
				client, err := constructor(ctx, "fixture", origin.URL, map[string]string{"X-API-Key": "redirect-fixture-secret"}, false)
				if client != nil {
					client.Close()
				}
				if err == nil {
					t.Fatal("redirected initialization should fail")
				}
				if received.Load() {
					t.Fatal("custom credential reached another origin through redirect")
				}
				if strings.Contains(err.Error(), "redirect-fixture-secret") {
					t.Fatal("rejected redirect URL leaked through error text")
				}
			})
		}
	}
}

func TestMCPRedirectOriginAndLimit(t *testing.T) {
	original, _ := http.NewRequest(http.MethodGet, "https://trusted.example/sse", nil)
	for _, target := range []string{
		"https://TRUSTED.example:443/message",
		"https://trusted.example/another",
	} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		if err := checkMCPRedirect(req, []*http.Request{original}); err != nil {
			t.Fatalf("same origin rejected: %v", err)
		}
	}
	for _, target := range []string{
		"http://trusted.example/message",
		"https://trusted.example:444/message",
		"https://other.example/message",
		"https://user:fixture@trusted.example/message",
	} {
		u, _ := url.Parse(target)
		if err := checkMCPRedirect(&http.Request{URL: u}, []*http.Request{original}); err == nil {
			t.Fatalf("unsafe redirect accepted: %s", target)
		}
	}
	if err := checkMCPRedirect(original, make([]*http.Request, 10)); err == nil {
		t.Fatal("10-hop limit not enforced")
	}
}

func TestMCPRedirectSameOriginHandshake(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/rpc", http.StatusTemporaryRedirect)
			return
		}
		if r.Header.Get("X-API-Key") != "fixture" {
			t.Error("same-origin redirect lost configured header")
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := New(ctx, "fixture", srv.URL+"/start", map[string]string{"X-API-Key": "fixture"}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if calls.Load() == 0 {
		t.Fatal("same-origin destination was not reached")
	}
}

func TestLegacyMessageRedirectDoesNotLeak(t *testing.T) {
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Store(true)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sse" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("event: endpoint\ndata: /message\n\n"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		http.Redirect(w, r, target.URL+"/message?token=legacy-secret", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := NewSSE(ctx, "fixture", origin.URL+"/sse", map[string]string{"X-API-Key": "fixture"}, false)
	if client != nil {
		client.Close()
	}
	if err == nil || reached.Load() || strings.Contains(err.Error(), "legacy-secret") {
		t.Fatalf("legacy message redirect not safely rejected: %v", err)
	}
}

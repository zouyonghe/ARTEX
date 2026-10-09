package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChannelRedirectEscapedQuoteNeverLeaksCredentials(t *testing.T) {
	for _, prefix := range []string{
		`/%zz?x="&access_token=`,
		`/%zz?x=\"&access_token=`,
		`/%zz?x="'&access_token=`,
	} {
		t.Run(prefix, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", prefix+leakProbeToken)
				w.WriteHeader(http.StatusTemporaryRedirect)
			}))
			defer srv.Close()

			_, err := (dingTalkChannel{}).Send(context.Background(),
				map[string]any{"webhook": srv.URL + "/robot/send"},
				Message{Items: []Item{{Severity: "high"}}})
			if err == nil {
				t.Fatal("invalid redirect target should fail")
			}
			assertNoSecret(t, err.Error(), leakProbeToken)
			if !strings.Contains(err.Error(), "invalid URL escape") {
				t.Fatalf("expected redirect parsing error, got %v", err)
			}
		})
	}
}

func TestRedactURLsInTextEscapedQuotes(t *testing.T) {
	for _, input := range []string{
		fmt.Sprintf("parse %q: invalid URL escape", `/%zz?x="&token=`+leakProbeToken),
		fmt.Sprintf("parse %q: invalid URL escape", `/%zz?x=\"&token=`+leakProbeToken),
		`parse '/%zz?x=\'&token=` + leakProbeToken + `': invalid URL escape`,
	} {
		got := redactURLsInText(input)
		assertNoSecret(t, got, leakProbeToken)
		if !strings.Contains(got, "invalid URL escape") {
			t.Fatalf("expected error category preserved, got %q", got)
		}
	}
	// A pair of escaped backslashes must not consume the closing quote or
	// suppress diagnostics that follow it.
	input := fmt.Sprintf("parse %q: connection refused", `/%zz?token=`+leakProbeToken+`\`)
	got := redactURLsInText(input)
	assertNoSecret(t, got, leakProbeToken)
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("expected trailing diagnostic preserved, got %q", got)
	}
}

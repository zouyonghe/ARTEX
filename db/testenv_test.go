package db

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeDatabaseIsNotInherited(t *testing.T) {
	if want := strings.TrimSpace(os.Getenv("ARTEX_TEST_PG_DSN")); want != "" {
		got, _, err := DSN()
		if err != nil || got != want {
			t.Fatal("database tests did not resolve the explicitly enabled test DSN")
		}
		return
	}
	if _, _, err := DSN(); err == nil {
		t.Fatal("database tests must not inherit runtime DSN or config; require ARTEX_TEST_PG_DSN")
	}
}

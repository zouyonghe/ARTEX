package db

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeDatabaseIsNotInherited(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ARTEX_TEST_PG_DSN")) != "" {
		t.Skip("default isolation check requires no explicit test database")
	}
	if _, _, err := DSN(); err == nil {
		t.Fatal("database tests must not inherit runtime DSN or config; require ARTEX_TEST_PG_DSN")
	}
}

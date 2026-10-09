package db

import (
	"database/sql"
	"os"
	"testing"

	"github.com/Autumn-27/artex/internal/testenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMain acquires a PostgreSQL advisory lock (7337741002) for the entire
// db test suite. Packages agent and server hold the same lock, so parallel
// `go test ./...` runs serialize across packages on the explicit test DB and
// avoid cross-package cleanup races (e.g. DELETE FROM assets WHERE id > X
// from one package deleting assets created by another).
func TestMain(m *testing.M) {
	os.Exit(testenv.Run(func() int { return runDBSuite(m) }))
}

func runDBSuite(m *testing.M) int {
	dsn, _, err := DSN()
	if err != nil {
		// No DB configured — tests that need PG will skip themselves.
		return m.Run()
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return m.Run()
	}
	defer conn.Close()
	if conn.Ping() != nil {
		return m.Run()
	}
	if _, err := conn.Exec(`SELECT pg_advisory_lock(7337741002)`); err != nil {
		return m.Run()
	}
	defer conn.Exec(`SELECT pg_advisory_unlock(7337741002)`) //nolint:errcheck
	return m.Run()
}

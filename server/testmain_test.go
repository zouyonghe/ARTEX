package server

import (
	"database/sql"
	"os"
	"testing"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/internal/testenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMain acquires a PostgreSQL advisory lock (7337741002) for the entire
// server test suite so cross-package DELETE cleanup races with db/agent
// packages are avoided when running `go test ./...`.
func TestMain(m *testing.M) {
	os.Exit(testenv.Run(func() int { return runServerSuite(m) }))
}

func runServerSuite(m *testing.M) int {
	dsn, _, err := db.DSN()
	if err != nil {
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

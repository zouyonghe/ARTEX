package llmrec

import (
	"database/sql"
	"os"
	"testing"

	"github.com/Autumn-27/artex/internal/testenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(func() int { return runLLMRecSuite(m) }))
}

// Coordinate cleanup with the other packages using the same disposable test DB.
func runLLMRecSuite(m *testing.M) int {
	dsn := os.Getenv("ARTEX_PG_DSN")
	if dsn == "" {
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

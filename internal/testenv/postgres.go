package testenv

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunPG isolates configuration and pins the suite lock to one PostgreSQL session.
// An explicitly enabled but unavailable database fails setup instead of skipping.
func RunPG(run func() int) int {
	return Run(func() int {
		dsn := os.Getenv("ARTEX_PG_DSN")
		if dsn == "" {
			return run()
		}
		pool, err := sql.Open("pgx", dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testenv: cannot open explicit test database")
			return 1
		}
		defer pool.Close()
		return withPostgresLock(pool, run)
	})
}

func withPostgresLock(pool *sql.DB, run func() int) (code int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pool.Conn(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv: cannot connect to explicit test database")
		return 1
	}
	defer conn.Close()
	if err := conn.PingContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "testenv: explicit test database is unavailable")
		return 1
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(7337741002)`); err != nil {
		fmt.Fprintln(os.Stderr, "testenv: cannot acquire PostgreSQL suite lock")
		return 1
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock(7337741002)`); err != nil {
			fmt.Fprintln(os.Stderr, "testenv: cannot release PostgreSQL suite lock")
			code = 1
			// Discard the physical connection so a failed unlock cannot retain the lock.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	return run()
}

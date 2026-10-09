package db

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOpenInvalidDSNDoesNotExposeCredentials(t *testing.T) {
	const secret = "PGCONFIGPROBE0123456789"
	// Invalid escapes or connect_timeout fail before dialing. The postgres
	// database path also prevents ensureDatabase's maintenance connection.
	for _, dsn := range []string{
		"postgres://user:" + secret + "@db.invalid/%zz",
		"postgres://db.invalid/%zz?password=" + secret,
		"postgres://user:" + secret + ":suffix@db.invalid/%zz",
		"postgres://db.invalid/postgres?password=" + secret + "&connect_timeout=not-a-number",
	} {
		t.Run(dsn, func(t *testing.T) {
			conn, err := Open(dsn)
			if conn != nil {
				conn.Close()
				t.Fatal("invalid DSN unexpectedly opened")
			}
			if err == nil {
				t.Fatal("invalid DSN should fail")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("virtual password leaked: %s", err)
			}
			var parseErr *pgconn.ParseConfigError
			if !errors.As(err, &parseErr) {
				t.Fatalf("parse error type lost: %T", err)
			}
		})
	}
}

func TestRedactPostgresConfigErrorPreservesErrorChain(t *testing.T) {
	cause := errors.New("fixture cause")
	parseErr := pgconn.NewParseConfigError("fixture-secret", "invalid fixture", cause)
	got := redactPostgresConfigError(parseErr)
	if strings.Contains(got.Error(), "fixture-secret") || !errors.Is(got, cause) {
		t.Fatal("parser error must hide DSN and retain underlying cause")
	}
	var typed *pgconn.ParseConfigError
	if !errors.As(got, &typed) {
		t.Fatal("parser error classification lost")
	}
	if got := redactPostgresConfigError(cause); got != cause {
		t.Fatal("unrelated error was changed")
	}
}

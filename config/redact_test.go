package config

import (
	"strings"
	"testing"
)

func TestRedactDSNDoesNotExposePassword(t *testing.T) {
	const secret = "DSNPROBE0123456789"
	for _, dsn := range []string{
		"postgres://user:" + secret + "@db.example/app?sslmode=require",
		"postgresql://db.example/app?password=" + secret,
		"postgres://db.example/app#" + secret,
		"host=db.example dbname=app user=user password='" + secret + "'",
		"postgres://user:" + secret + "@db.example/%zz",
		"postgres://user:fixture@db.example/app?sslpassword=" + secret + "&sslmode=require",
		"mysql://user:" + secret + "@db.example/app",
	} {
		t.Run(dsn, func(t *testing.T) {
			if got := Redact(dsn); strings.Contains(got, secret) {
				t.Fatalf("virtual password leaked: %q", got)
			}
		})
	}
}

func TestRedactDSNFailsClosed(t *testing.T) {
	for _, dsn := range []string{"", "postgres:///app", "password=fixture", "postgres://db.example/%zz"} {
		if got := Redact(dsn); got != "(DSN hidden)" {
			t.Fatalf("unrecognized DSN should be hidden, got %q", got)
		}
	}
}

func TestRedactDSNPreservesURLDiagnostics(t *testing.T) {
	got := Redact("postgres://user:fixture@db.example:5432/app?sslmode=require")
	for _, want := range []string{"postgres://", "user", "db.example:5432", "/app"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing diagnostic %q in %q", want, got)
		}
	}
}

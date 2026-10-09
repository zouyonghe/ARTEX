// Package testenv isolates PostgreSQL test suites from runtime configuration.
// It does not connect to a database and is not a security sandbox.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Run supplies a private empty config and only the explicitly opted-in test DSN
// for the duration of run. Call it before any suite setup, with os.Exit outside
// Run so environment restoration and temporary-file cleanup can execute.
func Run(run func() int) int {
	dir, err := os.MkdirTemp("", "artex-testenv-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv: cannot create isolated configuration")
		return 1
	}
	defer os.RemoveAll(dir)
	configPath, err := filepath.Abs(filepath.Join(dir, "config.json"))
	if err != nil || os.WriteFile(configPath, []byte("{}\n"), 0o600) != nil {
		fmt.Fprintln(os.Stderr, "testenv: cannot write isolated configuration")
		return 1
	}
	for _, setting := range []struct{ key, value string }{
		{"ARTEX_CONFIG", configPath},
		{"ARTEX_PG_DSN", strings.TrimSpace(os.Getenv("ARTEX_TEST_PG_DSN"))},
	} {
		key := setting.key
		old, exists := os.LookupEnv(key)
		defer func() {
			if exists {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		}()
		if err := os.Setenv(key, setting.value); err != nil {
			fmt.Fprintln(os.Stderr, "testenv: cannot isolate database environment")
			return 1
		}
	}
	return run()
}

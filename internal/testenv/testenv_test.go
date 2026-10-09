package testenv_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Autumn-27/artex/config"
	"github.com/Autumn-27/artex/internal/testenv"
)

func TestRunIsolatesRuntimeDatabase(t *testing.T) {
	for _, tc := range []struct {
		name, runtimeDSN, testDSN, want string
	}{
		{"runtime env", "postgres://runtime:fixture@runtime.invalid/runtime", "", ""},
		{"runtime config", "", "", ""},
		{"whitespace", "postgres://runtime.invalid/runtime", " \t\n ", ""},
		{"explicit opt in", "postgres://runtime.invalid/runtime", " postgres://test.invalid/disposable \n", "postgres://test.invalid/disposable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			t.Setenv("TMP", tmp)
			t.Setenv("TEMP", tmp)
			runtimeConfig := filepath.Join(tmp, "runtime.json")
			if err := os.WriteFile(runtimeConfig, []byte(`{"database":{"dsn":"postgres://runtime-config.invalid/production"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ARTEX_CONFIG", runtimeConfig)
			t.Setenv("ARTEX_PG_DSN", tc.runtimeDSN)
			t.Setenv("ARTEX_TEST_PG_DSN", tc.testDSN)
			var isolated string
			called := false
			code := testenv.Run(func() int {
				called = true
				isolated = os.Getenv("ARTEX_CONFIG")
				if isolated == runtimeConfig || !filepath.IsAbs(isolated) {
					t.Fatalf("config not isolated: %q", isolated)
				}
				info, err := os.Stat(isolated)
				if err != nil {
					t.Fatal(err)
				}
				// Windows reports only the read-only attribute, not Unix mode bits.
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
					t.Fatalf("config permissions = %o", info.Mode().Perm())
				}
				got, _, err := config.PostgresDSN()
				if tc.want == "" {
					if err == nil || got != "" {
						t.Fatal("runtime database was inherited")
					}
				} else if err != nil || got != tc.want {
					t.Fatal("explicit test DSN not selected")
				}
				return 7
			})
			if !called || code != 7 {
				t.Fatalf("runner invoked=%v exit=%d", called, code)
			}
			if _, err := os.Stat(filepath.Dir(isolated)); !os.IsNotExist(err) {
				t.Fatal("isolated directory not removed")
			}
			if os.Getenv("ARTEX_CONFIG") != runtimeConfig || os.Getenv("ARTEX_PG_DSN") != tc.runtimeDSN {
				t.Fatal("runtime environment not restored")
			}
		})
	}
}

func TestRunFailsClosedWithoutTempDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("TMPDIR", missing)
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	t.Setenv("ARTEX_PG_DSN", "runtime-fixture")
	called := false
	code := testenv.Run(func() int { called = true; return 0 })
	if called || code == 0 {
		t.Fatal("runner executed without isolated configuration")
	}
	if os.Getenv("ARTEX_PG_DSN") != "runtime-fixture" {
		t.Fatal("setup failure changed runtime environment")
	}
}

func TestRunRestoresUnsetEnvironmentAfterPanic(t *testing.T) {
	for _, key := range []string{"ARTEX_CONFIG", "ARTEX_PG_DSN"} {
		t.Setenv(key, "fixture") // Let testing restore the original process environment.
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ARTEX_TEST_PG_DSN", "")
	var isolated string
	func() {
		defer func() {
			if recover() != "fixture panic" {
				t.Fatal("runner panic not propagated")
			}
		}()
		testenv.Run(func() int {
			isolated = os.Getenv("ARTEX_CONFIG")
			panic("fixture panic")
		})
	}()
	for _, key := range []string{"ARTEX_CONFIG", "ARTEX_PG_DSN"} {
		if _, exists := os.LookupEnv(key); exists {
			t.Fatalf("previously unset %s was not restored", key)
		}
	}
	if _, err := os.Stat(filepath.Dir(isolated)); !os.IsNotExist(err) {
		t.Fatal("isolated directory not removed after panic")
	}
}

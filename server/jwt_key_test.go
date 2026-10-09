package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJWTKeyReadErrorIsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, jwtKeyFilename)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateJWTKey(dir, dir)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "open" && pathErr.Op != "read" {
		t.Fatalf("read cause not preserved: %v", err)
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatal("unreadable key path changed")
	}
}

func TestJWTKeyInvalidFileIsNotReplaced(t *testing.T) {
	for _, contents := range []string{"", "short-fixture", " \n\t "} {
		t.Run(contents, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, jwtKeyFilename)
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadOrCreateJWTKey(dir, dir); err == nil {
				t.Fatal("invalid existing key must fail rather than rotate")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != contents {
				t.Fatal("existing key content changed")
			}
		})
	}
}

func TestJWTKeyCreateAndReuse(t *testing.T) {
	dir := t.TempDir()
	first, err := loadOrCreateJWTKey(dir, dir)
	if err != nil || len(first) != 32 {
		t.Fatalf("create key: length=%d error=%v", len(first), err)
	}
	second, err := loadOrCreateJWTKey(dir, dir)
	if err != nil || string(first) != string(second) {
		t.Fatal("existing valid key not reused")
	}
	path := filepath.Join(dir, jwtKeyFilename)
	if err := os.WriteFile(path, []byte("\n"+strings.Repeat("fixture", 6)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := loadOrCreateJWTKey(dir, dir)
	if err != nil || string(key) != strings.Repeat("fixture", 6) {
		t.Fatal("valid key whitespace compatibility lost")
	}
}

func TestJWTKeyLegacyMigrationPreservesContents(t *testing.T) {
	for _, size := range []int{31, 32} {
		t.Run(strings.Repeat("x", size), func(t *testing.T) {
			keyDir, dataDir := t.TempDir(), t.TempDir()
			contents := strings.Repeat("x", size)
			legacy := filepath.Join(dataDir, jwtKeyFilename)
			if err := os.WriteFile(legacy, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			key, err := loadOrCreateJWTKey(keyDir, dataDir)
			if (err != nil) != (size < 32) {
				t.Fatalf("length=%d error=%v", size, err)
			}
			if size == 32 && string(key) != contents {
				t.Fatal("migrated signing key changed")
			}
			got, readErr := os.ReadFile(filepath.Join(keyDir, jwtKeyFilename))
			if readErr != nil || string(got) != contents {
				t.Fatal("migration replaced original bytes")
			}
			if _, err := os.Stat(legacy); !os.IsNotExist(err) {
				t.Fatal("legacy key not removed after migration")
			}
		})
	}
}

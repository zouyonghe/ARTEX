package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJWTKeyUnreadableLegacyDoesNotGenerateReplacement(t *testing.T) {
	keyDir, dataDir := t.TempDir(), t.TempDir()
	legacy := filepath.Join(dataDir, jwtKeyFilename)
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	key, err := loadOrCreateJWTKey(keyDir, dataDir)
	var pathErr *os.PathError
	if key != nil || !errors.As(err, &pathErr) {
		t.Fatal("unreadable legacy key must return its read error, not generate a new key")
	}
	if pathErr.Path != legacy {
		t.Fatal("read error does not identify the legacy path")
	}
	if _, err := os.Stat(filepath.Join(keyDir, jwtKeyFilename)); !os.IsNotExist(err) {
		t.Fatal("replacement key was created")
	}
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		t.Fatal("legacy path changed")
	}
}

func TestJWTKeyMigrationWriteErrorKeepsLegacy(t *testing.T) {
	keyDir := filepath.Join(t.TempDir(), "missing")
	dataDir := t.TempDir()
	legacy := filepath.Join(dataDir, jwtKeyFilename)
	contents := "fixture-key-material-0123456789012345"
	if err := os.WriteFile(legacy, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateJWTKey(keyDir, dataDir)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != filepath.Join(keyDir, jwtKeyFilename) {
		t.Fatalf("migration write cause not preserved: %v", err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || string(got) != contents {
		t.Fatal("legacy key changed after write failure")
	}
}

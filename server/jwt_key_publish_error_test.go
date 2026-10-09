package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJWTKeyInvalidPublishTargetIsNotCalledConcurrentCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, jwtKeyFilename)
	const contents = "fixture-short-key"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateJWTKey(dir, dir)
	if err == nil || !strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "concurrently created") {
		t.Fatal("pre-existing invalid key has misleading diagnostic")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != contents {
		t.Fatal("invalid target was replaced")
	}
}

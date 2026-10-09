package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentJWTLegacyMigrationUsesOneKey(t *testing.T) {
	keyDir, dataDir := t.TempDir(), t.TempDir()
	const callers = 32
	contents := strings.Repeat("legacy-fixture", 3)
	legacy := filepath.Join(dataDir, jwtKeyFilename)
	if err := os.WriteFile(legacy, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	keys := make([][]byte, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			keys[i], errs[i] = loadOrCreateJWTKey(keyDir, dataDir)
		}()
	}
	close(start)
	wg.Wait()
	for i := range callers {
		if errs[i] != nil || string(keys[i]) != contents {
			t.Fatalf("migration caller %d returned a different key or failed: %v", i, errs[i])
		}
	}
	disk, err := os.ReadFile(filepath.Join(keyDir, jwtKeyFilename))
	if err != nil || string(disk) != contents {
		t.Fatal("legacy signing material changed")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("completed migration retained legacy file")
	}
}

package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentJWTKeyCreationUsesOneKey(t *testing.T) {
	dir := t.TempDir()
	const callers = 32
	start := make(chan struct{})
	keys := make([][]byte, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			keys[i], errs[i] = loadOrCreateJWTKey(dir, dir)
		}()
	}
	close(start)
	wg.Wait()
	disk, err := os.ReadFile(filepath.Join(dir, jwtKeyFilename))
	if err != nil {
		t.Fatal(err)
	}
	for i := range callers {
		if errs[i] != nil {
			t.Errorf("caller %d failed: %v", i, errs[i])
		} else if string(keys[i]) != string(disk) {
			t.Errorf("caller %d received a different key from the persisted key", i)
		}
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".jwt-key-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary key files not cleaned: %d, error=%v", len(leftovers), err)
	}
}

func TestJWTPublicationDoesNotReplaceDifferentWinner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, jwtKeyFilename)
	winner := strings.Repeat("winner-fixture", 3)
	if err := os.WriteFile(path, []byte(winner), 0o600); err != nil {
		t.Fatal(err)
	}
	key, created, err := publishJWTKey(path, []byte(strings.Repeat("legacy-fixture", 3)))
	if err != nil || created || string(key) != winner {
		t.Fatal("publication did not preserve the existing winner")
	}
	disk, err := os.ReadFile(path)
	if err != nil || string(disk) != winner {
		t.Fatal("publication overwrote destination key")
	}
}

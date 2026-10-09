package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// publishJWTKey installs complete key material without replacing an existing key.
// The boolean reports whether this caller published, rather than reused, the file.
func publishJWTKey(path string, data []byte) ([]byte, bool, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".jwt-key-*")
	if err != nil {
		return nil, false, fmt.Errorf("create jwt key temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, false, fmt.Errorf("write jwt key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, false, fmt.Errorf("close jwt key: %w", err)
	}
	if err := os.Link(tmp.Name(), path); err != nil {
		if !os.IsExist(err) {
			return nil, false, fmt.Errorf("publish jwt key (filesystem must support hard links): %w", err)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, false, fmt.Errorf("read existing jwt key: %w", readErr)
		}
		key := strings.TrimSpace(string(data))
		if len(key) < 32 {
			return nil, false, fmt.Errorf("existing jwt key is invalid (not replaced); restore a trusted key before restarting")
		}
		return []byte(key), false, nil
	}
	return []byte(strings.TrimSpace(string(data))), true, nil
}

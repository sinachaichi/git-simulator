// Package testutil has small helpers shared by the tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteFiles creates files below root. The map key is a slash path.
func WriteFiles(t testing.TB, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

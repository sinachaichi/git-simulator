// Package testutil has small helpers shared by the tests.
package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

// CopyDir copies a directory tree. The differential tests use it to run
// real git on a copy of a simulated repository.
func CopyDir(t testing.TB, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// SkipWithoutExecBit skips a test on systems that have no executable
// file mode, such as windows.
func SkipWithoutExecBit(t testing.TB) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("this system has no executable bit")
	}
}

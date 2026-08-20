// Package workdir gives access to the files of a simulated working tree.
// Paths going in and out are always relative and use forward slashes.
package workdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
)

// MetaDir is the name of the directory that holds the simulated repository
// data. It is never part of the working tree.
const MetaDir = ".hood"

// WorkDir is a directory on disk that acts as a working tree.
type WorkDir struct {
	Root string
	// Observer, when set, hears about files written or removed.
	Observer events.Sink
}

func (w *WorkDir) emit(e events.Event) {
	if w.Observer != nil {
		w.Observer.Emit(e)
	}
}

// New returns a WorkDir for root. The directory must already exist.
func New(root string) (*WorkDir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}
	return &WorkDir{Root: abs}, nil
}

// abs turns a relative slash path into an absolute OS path. It rejects
// paths that would leave the working tree or point into the meta dir.
func (w *WorkDir) abs(rel string) (string, error) {
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("path %q is outside the working tree", rel)
	}
	if clean == MetaDir || strings.HasPrefix(clean, MetaDir+"/") {
		return "", fmt.Errorf("path %q is inside %s", rel, MetaDir)
	}
	return filepath.Join(w.Root, filepath.FromSlash(clean)), nil
}

// ReadFile returns the content of a file.
func (w *WorkDir) ReadFile(rel string) ([]byte, error) {
	p, err := w.abs(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// WriteFile replaces the content of a file and creates parent directories.
func (w *WorkDir) WriteFile(rel string, data []byte, mode fs.FileMode) error {
	p, err := w.abs(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(p, mode); err != nil {
		return err
	}
	w.emit(events.Event{Kind: events.FileWritten, Subject: path.Clean(rel), Detail: fmt.Sprintf("%d bytes", len(data))})
	return nil
}

// AppendFile adds data to the end of an existing file.
func (w *WorkDir) AppendFile(rel string, data []byte) error {
	p, err := w.abs(rel)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Close())
}

// Remove deletes a file and any parent directories left empty.
func (w *WorkDir) Remove(rel string) error {
	p, err := w.abs(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	w.emit(events.Event{Kind: events.FileRemoved, Subject: path.Clean(rel)})
	for dir := filepath.Dir(p); dir != w.Root; dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// Exists reports whether a regular file exists at rel.
func (w *WorkDir) Exists(rel string) bool {
	info, err := w.Stat(rel)
	return err == nil && info.Mode().IsRegular()
}

// Stat returns file info for rel.
func (w *WorkDir) Stat(rel string) (fs.FileInfo, error) {
	p, err := w.abs(rel)
	if err != nil {
		return nil, err
	}
	return os.Lstat(p)
}

// List returns every regular file in the tree, sorted, without the meta dir.
func (w *WorkDir) List() ([]string, error) {
	return w.ListIn(".")
}

// ListIn returns the regular files below dir. Use "." for the whole tree.
func (w *WorkDir) ListIn(dir string) ([]string, error) {
	start := w.Root
	if dir != "." {
		p, err := w.abs(dir)
		if err != nil {
			return nil, err
		}
		start = p
	}
	var files []string
	err := filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && p == start {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(w.Root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == MetaDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// Package refs stores branches, tags and HEAD as plain files, the same
// way git does for loose refs.
package refs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

// Prefixes of the two ref namespaces we use.
const (
	BranchPrefix = "refs/heads/"
	TagPrefix    = "refs/tags/"
)

// ErrNotFound is returned when a ref does not exist.
var ErrNotFound = errors.New("ref not found")

// Ref is a name and the object it points to.
type Ref struct {
	Name string
	ID   objects.ID
}

// Store reads and writes refs below Dir (usually ".hood").
type Store struct {
	Dir string
	// Observer, when set, hears about every change.
	Observer events.Sink
}

func (s *Store) emit(e events.Event) {
	if s.Observer != nil {
		s.Observer.Emit(e)
	}
}

// ValidName checks a full ref name such as "refs/heads/feat/x" against
// the main rules of `git check-ref-format`.
func ValidName(name string) error {
	bad := func(why string) error { return fmt.Errorf("invalid ref name %q: %s", name, why) }
	if name == "" || name == "@" {
		return bad("it is empty or just @")
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return bad("slashes must separate non-empty parts")
	}
	if strings.HasSuffix(name, ".") {
		return bad("it cannot end with a dot")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") {
		return bad("it cannot contain .. or @{")
	}
	if strings.ContainsAny(name, " ~^:?*[\\\x7f") {
		return bad("it contains a forbidden character")
	}
	for _, r := range name {
		if r < 0x20 {
			return bad("it contains a control character")
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return bad("a part cannot start with . or end with .lock")
		}
	}
	return nil
}

func (s *Store) path(name string) string {
	return filepath.Join(s.Dir, filepath.FromSlash(name))
}

// Read returns the id a ref points to.
func (s *Store) Read(name string) (objects.ID, error) {
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return objects.Zero, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return objects.Zero, err
	}
	id, err := objects.ParseID(strings.TrimSpace(string(data)))
	if err != nil {
		return objects.Zero, fmt.Errorf("ref %s is corrupt: %w", name, err)
	}
	return id, nil
}

// Exists reports whether a ref exists.
func (s *Store) Exists(name string) bool {
	_, err := s.Read(name)
	return err == nil
}

// Update points a ref at id. The write goes through a lock file so two
// writers cannot interleave.
func (s *Store) Update(name string, id objects.ID) error {
	if err := ValidName(name); err != nil {
		return err
	}
	old, _ := s.Read(name)
	if err := s.writeFile(name, id.String()+"\n"); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.RefUpdated, Subject: name, Old: old.String(), New: id.String()})
	return nil
}

func (s *Store) writeFile(name, content string) error {
	p := s.path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	lock := p + ".lock"
	f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("cannot lock %s: %w", name, err)
	}
	_, err = f.WriteString(content)
	err = errors.Join(err, f.Close())
	if err == nil {
		err = os.Rename(lock, p)
	}
	if err != nil {
		return errors.Join(err, os.Remove(lock))
	}
	return nil
}

// Delete removes a ref and any directories it leaves empty.
func (s *Store) Delete(name string) error {
	old, _ := s.Read(name)
	err := os.Remove(s.path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.RefDeleted, Subject: name, Old: old.String()})
	for dir := filepath.Dir(s.path(name)); dir != s.Dir; dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// List returns all refs whose name starts with prefix, sorted by name.
func (s *Store) List(prefix string) ([]Ref, error) {
	root := s.path(strings.TrimSuffix(prefix, "/"))
	var out []Ref
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || strings.HasSuffix(p, ".lock") {
			return nil
		}
		rel, err := filepath.Rel(s.Dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		id, err := s.Read(name)
		if err != nil {
			return err
		}
		out = append(out, Ref{Name: name, ID: id})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

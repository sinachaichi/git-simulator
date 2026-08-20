// Package odb is the loose object database: one zlib compressed file per
// object under objects/<2 hex>/<38 hex>, the same layout git uses.
package odb

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("object not found")

// AmbiguousError is returned when a short id matches several objects.
type AmbiguousError struct {
	Prefix     string
	Candidates []objects.ID
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("short id %s is ambiguous (%d objects match)", e.Prefix, len(e.Candidates))
}

// Store reads and writes objects below Dir (usually ".hood/objects").
type Store struct {
	Dir string
	// Observer, when set, hears about every write.
	Observer events.Sink
}

// New returns a Store and makes sure the directory exists.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func (s *Store) emit(e events.Event) {
	if s.Observer != nil {
		s.Observer.Emit(e)
	}
}

func (s *Store) path(id objects.ID) string {
	h := id.String()
	return filepath.Join(s.Dir, h[:2], h[2:])
}

// Has reports whether the object exists.
func (s *Store) Has(id objects.ID) bool {
	_, err := os.Stat(s.path(id))
	return err == nil
}

// Write stores an object and returns its id. created is false when the
// object was already there, which is how git deduplicates content.
func (s *Store) Write(o objects.Object) (id objects.ID, created bool, err error) {
	id = objects.HashOf(o)
	detail := fmt.Sprintf("%s %d bytes", o.Type(), len(o.Body()))
	if s.Has(id) {
		s.emit(events.Event{Kind: events.ObjectExists, Subject: id.String(), Detail: detail})
		return id, false, nil
	}
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(objects.Serialize(o)); err != nil {
		return id, false, err
	}
	if err := zw.Close(); err != nil {
		return id, false, err
	}
	p := s.path(id)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return id, false, err
	}
	// Write to a temp file first so a crash never leaves a half object.
	tmp, err := os.CreateTemp(filepath.Dir(p), "tmp_obj_")
	if err != nil {
		return id, false, err
	}
	_, err = tmp.Write(buf.Bytes())
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o444)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	}
	if err != nil {
		return id, false, errors.Join(err, os.Remove(tmp.Name()))
	}
	s.emit(events.Event{Kind: events.ObjectWritten, Subject: id.String(), Detail: detail})
	return id, true, nil
}

// Read loads an object and checks that its content still hashes to id.
func (s *Store) Read(id objects.ID) (objects.Object, error) {
	f, err := os.Open(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	zr, err := zlib.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("object %s is corrupt: %w", id, err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("object %s is corrupt: %w", id, err)
	}
	o, err := objects.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("object %s: %w", id, err)
	}
	if got := objects.HashOf(o); got != id {
		return nil, fmt.Errorf("object %s is corrupt: content hashes to %s", id, got)
	}
	return o, nil
}

// All lists every object id in the store.
func (s *Store) All() ([]objects.ID, error) {
	dirs, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var ids []objects.ID
	for _, d := range dirs {
		if !d.IsDir() || len(d.Name()) != 2 {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.Dir, d.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			id, err := objects.ParseID(d.Name() + f.Name())
			if err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

// Resolve expands a hex prefix of 4 to 40 characters into a full id.
func (s *Store) Resolve(prefix string) (objects.ID, error) {
	prefix = strings.ToLower(prefix)
	if len(prefix) < 4 || len(prefix) > 40 || !isHex(prefix) {
		return objects.Zero, fmt.Errorf("%q is not a valid short id (use 4 to 40 hex characters)", prefix)
	}
	ids, err := s.All()
	if err != nil {
		return objects.Zero, err
	}
	var matches []objects.ID
	for _, id := range ids {
		if strings.HasPrefix(id.String(), prefix) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return objects.Zero, fmt.Errorf("%w: %s", ErrNotFound, prefix)
	case 1:
		return matches[0], nil
	}
	return objects.Zero, &AmbiguousError{Prefix: prefix, Candidates: matches}
}

func isHex(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// ReadBlob reads an object and checks that it is a blob.
func (s *Store) ReadBlob(id objects.ID) (*objects.Blob, error) {
	o, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	b, ok := o.(*objects.Blob)
	if !ok {
		return nil, wrongType(id, objects.TypeBlob, o)
	}
	return b, nil
}

// ReadTree reads an object and checks that it is a tree.
func (s *Store) ReadTree(id objects.ID) (*objects.Tree, error) {
	o, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	t, ok := o.(*objects.Tree)
	if !ok {
		return nil, wrongType(id, objects.TypeTree, o)
	}
	return t, nil
}

// ReadCommit reads an object and checks that it is a commit.
func (s *Store) ReadCommit(id objects.ID) (*objects.Commit, error) {
	o, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	c, ok := o.(*objects.Commit)
	if !ok {
		return nil, wrongType(id, objects.TypeCommit, o)
	}
	return c, nil
}

func wrongType(id objects.ID, want objects.Type, got objects.Object) error {
	return fmt.Errorf("object %s is a %s, not a %s", id.Short(), got.Type(), want)
}

// Abbrev returns the shortest unique prefix of id that is at least min
// characters long.
func (s *Store) Abbrev(id objects.ID, minLen int) (string, error) {
	ids, err := s.All()
	if err != nil {
		return "", err
	}
	full := id.String()
	n := minLen
	for _, other := range ids {
		if other == id {
			continue
		}
		for n < len(full) && strings.HasPrefix(other.String(), full[:n]) {
			n++
		}
	}
	return full[:n], nil
}

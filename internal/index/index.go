package index

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// Index is the in-memory staging area. Entries are kept sorted by path
// and stage.
type Index struct {
	entries []Entry
}

// New returns an empty index.
func New() *Index { return &Index{} }

// Load reads an index file. A missing file gives an empty index, like a
// repo where nothing was ever staged.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return &Index{entries: entries}, nil
}

// Save writes the index to path.
func (ix *Index) Save(path string) error {
	return os.WriteFile(path, Marshal(ix.entries), 0o644)
}

// Entries returns a copy of all entries, including conflict stages.
func (ix *Index) Entries() []Entry {
	return append([]Entry(nil), ix.entries...)
}

// Len returns the number of entries.
func (ix *Index) Len() int { return len(ix.entries) }

// find returns the position where (path, stage) is or would be inserted.
func (ix *Index) find(path string, stage uint8) (int, bool) {
	i := sort.Search(len(ix.entries), func(i int) bool {
		e := ix.entries[i]
		return e.Path > path || (e.Path == path && e.Stage >= stage)
	})
	return i, i < len(ix.entries) && ix.entries[i].Path == path && ix.entries[i].Stage == stage
}

// Get returns the stage 0 entry for path.
func (ix *Index) Get(path string) (Entry, bool) {
	i, ok := ix.find(path, 0)
	if !ok {
		return Entry{}, false
	}
	return ix.entries[i], true
}

// Add stages an entry at stage 0. It clears conflict stages for the same
// path, and removes entries that would clash with it: a file named "a"
// replaces everything under "a/", and "a/b" replaces a file named "a".
func (ix *Index) Add(e Entry) {
	e.Stage = 0
	ix.removeClashes(e.Path)
	ix.removeStages(e.Path)
	i, _ := ix.find(e.Path, 0)
	ix.entries = append(ix.entries, Entry{})
	copy(ix.entries[i+1:], ix.entries[i:])
	ix.entries[i] = e
}

func (ix *Index) removeClashes(path string) {
	kept := ix.entries[:0]
	for _, e := range ix.entries {
		underNew := strings.HasPrefix(e.Path, path+"/")
		parentOfNew := strings.HasPrefix(path, e.Path+"/")
		if !underNew && !parentOfNew {
			kept = append(kept, e)
		}
	}
	ix.entries = kept
}

func (ix *Index) removeStages(path string) {
	kept := ix.entries[:0]
	for _, e := range ix.entries {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	ix.entries = kept
}

// Remove deletes every entry (all stages) for path and reports whether
// anything was removed.
func (ix *Index) Remove(path string) bool {
	before := len(ix.entries)
	ix.removeStages(path)
	return len(ix.entries) != before
}

// Paths returns the sorted, unique paths in the index.
func (ix *Index) Paths() []string {
	var out []string
	for _, e := range ix.entries {
		if len(out) == 0 || out[len(out)-1] != e.Path {
			out = append(out, e.Path)
		}
	}
	return out
}

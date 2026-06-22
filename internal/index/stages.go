package index

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// UnmergedError is returned when an operation needs a clean index but
// some paths still have conflict stages.
type UnmergedError struct {
	Paths []string
}

func (e *UnmergedError) Error() string {
	return fmt.Sprintf("unmerged paths in the index: %s", strings.Join(e.Paths, ", "))
}

// SetConflict replaces the entries for path with the three sides of a
// conflict. Any side may be nil: a file added on both branches has no
// base, and a file deleted on one side has no "ours" or "theirs".
func (ix *Index) SetConflict(path string, base, ours, theirs *Entry) {
	ix.removeStages(path)
	for stage, e := range []*Entry{base, ours, theirs} {
		if e == nil {
			continue
		}
		entry := *e
		entry.Path = path
		entry.Stage = uint8(stage + 1)
		i, _ := ix.find(path, entry.Stage)
		ix.entries = append(ix.entries, Entry{})
		copy(ix.entries[i+1:], ix.entries[i:])
		ix.entries[i] = entry
	}
}

// Stages returns the conflict entries for path, indexed by stage - 1.
// Missing sides are nil.
func (ix *Index) Stages(path string) [3]*Entry {
	var out [3]*Entry
	for stage := uint8(1); stage <= 3; stage++ {
		if i, ok := ix.find(path, stage); ok {
			e := ix.entries[i]
			out[stage-1] = &e
		}
	}
	return out
}

// Conflicts returns the sorted paths that have conflict stages.
func (ix *Index) Conflicts() []string {
	seen := map[string]bool{}
	for _, e := range ix.entries {
		if e.Stage != 0 {
			seen[e.Path] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// FromTree builds an index that matches a tree exactly.
func FromTree(s *odb.Store, id objects.ID) (*Index, error) {
	files, err := tree.Flatten(s, id)
	if err != nil {
		return nil, err
	}
	ix := New()
	for _, f := range files {
		ix.entries = append(ix.entries, Entry{Path: f.Path, Mode: f.Mode, ID: f.ID})
	}
	return ix, nil
}

// WriteTree stores the index as tree objects and returns the root id. It
// refuses to run while there are unmerged paths.
func (ix *Index) WriteTree(s *odb.Store) (objects.ID, error) {
	if conflicts := ix.Conflicts(); len(conflicts) > 0 {
		return objects.Zero, &UnmergedError{Paths: conflicts}
	}
	files := make([]tree.File, len(ix.entries))
	for i, e := range ix.entries {
		files[i] = tree.File{Path: e.Path, Mode: e.Mode, ID: e.ID}
	}
	return tree.Build(s, files)
}

// Package tree converts between nested tree objects and flat file lists.
// Most of the code (index, diff, merge) is easier to write on flat paths.
package tree

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
)

// File is one file in a snapshot.
type File struct {
	Path string
	Mode uint32
	ID   objects.ID
}

// Flatten walks a tree and returns every file with its full slash path.
// The result is in git's path order, the same order the index uses.
func Flatten(s *odb.Store, id objects.ID) ([]File, error) {
	var out []File
	if err := walk(s, id, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func walk(s *odb.Store, id objects.ID, prefix string, out *[]File) error {
	t, err := s.ReadTree(id)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		if e.IsDir() {
			if err := walk(s, e.ID, prefix+e.Name+"/", out); err != nil {
				return err
			}
			continue
		}
		*out = append(*out, File{Path: prefix + e.Name, Mode: e.Mode, ID: e.ID})
	}
	return nil
}

// Build writes the trees for a list of files and returns the root id.
// It fails if a path is both a file and a directory.
func Build(s *odb.Store, files []File) (objects.ID, error) {
	sorted := append([]File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	return build(s, sorted, "")
}

// build writes the tree for files, whose paths all start with prefix.
func build(s *odb.Store, files []File, prefix string) (objects.ID, error) {
	t := &objects.Tree{}
	for i := 0; i < len(files); {
		rest := strings.TrimPrefix(files[i].Path, prefix)
		name, _, nested := strings.Cut(rest, "/")
		if !nested {
			t.Entries = append(t.Entries, objects.TreeEntry{Mode: files[i].Mode, Name: name, ID: files[i].ID})
			i++
			continue
		}
		j := i
		for j < len(files) && strings.HasPrefix(files[j].Path, prefix+name+"/") {
			j++
		}
		sub, err := build(s, files[i:j], prefix+name+"/")
		if err != nil {
			return objects.Zero, err
		}
		t.Entries = append(t.Entries, objects.TreeEntry{Mode: objects.ModeDir, Name: name, ID: sub})
		i = j
	}
	for i := 1; i < len(t.Entries); i++ {
		if t.Entries[i].Name == t.Entries[i-1].Name {
			return objects.Zero, fmt.Errorf("path %s%s is both a file and a directory", prefix, t.Entries[i].Name)
		}
	}
	t.Sort()
	id, _, err := s.Write(t)
	return id, err
}

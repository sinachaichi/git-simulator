// Package porcelain implements the user level commands (add, commit,
// status and so on) on top of the repo package. Functions return plain
// result values and leave all printing to the cli package.
package porcelain

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// PathspecError is returned when a path matches nothing.
type PathspecError struct {
	Spec string
}

func (e *PathspecError) Error() string {
	return fmt.Sprintf("pathspec '%s' did not match any files", e.Spec)
}

// AddResult lists what Add changed in the index.
type AddResult struct {
	Added    []string
	Modified []string
	Removed  []string
}

// Add stages paths. Each spec is a file, a directory or "." for
// everything, relative to the repo root with forward slashes. A tracked
// file that no longer exists is staged as a deletion.
func Add(r *repo.Repo, specs []string) (AddResult, error) {
	var res AddResult
	if len(specs) == 0 {
		return res, fmt.Errorf("nothing specified, nothing added")
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return res, err
	}
	files, err := r.Work.List()
	if err != nil {
		return res, err
	}
	paths := map[string]bool{}
	for _, spec := range specs {
		spec = cleanSpec(spec)
		matched := false
		for _, p := range append(append([]string(nil), files...), ix.Paths()...) {
			if matches(spec, p) {
				paths[p] = true
				matched = true
			}
		}
		if !matched {
			return res, &PathspecError{Spec: spec}
		}
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)

	for _, p := range ordered {
		entry, err := stageFile(r, p)
		old, tracked := ix.Get(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if ix.Remove(p) {
				res.Removed = append(res.Removed, p)
			}
		case err != nil:
			return res, err
		case !tracked:
			ix.Add(entry)
			res.Added = append(res.Added, p)
		case old.ID != entry.ID || old.Mode != entry.Mode:
			ix.Add(entry)
			res.Modified = append(res.Modified, p)
		default:
			// Unchanged content: refresh the stat data only.
			ix.Add(entry)
		}
	}
	return res, r.SaveIndex(ix)
}

func cleanSpec(spec string) string {
	spec = strings.TrimPrefix(strings.ReplaceAll(spec, "\\", "/"), "./")
	spec = strings.TrimSuffix(spec, "/")
	if spec == "" {
		return "."
	}
	return spec
}

func matches(spec, path string) bool {
	return spec == "." || path == spec || strings.HasPrefix(path, spec+"/")
}

// readWorkFile reads a working tree file once and returns its content
// with the index entry it would get.
func readWorkFile(r *repo.Repo, path string) (index.Entry, []byte, error) {
	data, err := r.Work.ReadFile(path)
	if err != nil {
		return index.Entry{}, nil, err
	}
	info, err := r.Work.Stat(path)
	if err != nil {
		return index.Entry{}, nil, err
	}
	mode := objects.ModeFile
	if info.Mode()&0o111 != 0 {
		mode = objects.ModeExecutable
	}
	return index.Entry{
		Path: path, Mode: mode, Size: uint32(len(data)), MTime: info.ModTime(),
		ID: objects.HashOf(&objects.Blob{Data: data}),
	}, data, nil
}

// hashWorkFile computes the entry for a file without storing anything.
// Status uses it to compare against the index.
func hashWorkFile(r *repo.Repo, path string) (index.Entry, error) {
	entry, _, err := readWorkFile(r, path)
	return entry, err
}

// stageFile writes the blob for a working tree file and returns the
// index entry for it.
func stageFile(r *repo.Repo, path string) (index.Entry, error) {
	entry, data, err := readWorkFile(r, path)
	if err != nil {
		return index.Entry{}, err
	}
	if _, _, err := r.ODB.Write(&objects.Blob{Data: data}); err != nil {
		return index.Entry{}, err
	}
	return entry, nil
}

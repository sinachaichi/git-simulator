package porcelain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// DiffOptions are the flags of `diff`.
type DiffOptions struct {
	Staged bool     // --staged: compare against the index instead of the working tree
	Revs   []string // zero, one or two revisions
	Paths  []string // limit the output to these paths or directories
}

// side is one side of a comparison: the files it has, and how to read
// their content.
type side map[string]sideFile

type sideFile struct {
	mode uint32
	id   objects.ID
	read func() ([]byte, error)
}

// Diff compares two states of the repository and returns one entry per
// changed file, sorted by path:
//
//	diff                 index vs working tree
//	diff --staged        HEAD vs index
//	diff <rev>           commit vs working tree
//	diff --staged <rev>  commit vs index
//	diff <rev> <rev>     commit vs commit
func Diff(r *repo.Repo, opts DiffOptions) ([]diff.File, error) {
	if len(opts.Revs) > 2 {
		return nil, fmt.Errorf("diff takes at most two revisions, got %d", len(opts.Revs))
	}
	if len(opts.Revs) == 2 && opts.Staged {
		return nil, fmt.Errorf("--staged cannot be combined with two revisions")
	}
	var oldSide, newSide side
	var err error
	switch {
	case len(opts.Revs) == 2:
		if oldSide, err = commitSide(r, opts.Revs[0]); err != nil {
			return nil, err
		}
		newSide, err = commitSide(r, opts.Revs[1])
	case opts.Staged:
		rev := "HEAD"
		if len(opts.Revs) == 1 {
			rev = opts.Revs[0]
		}
		if oldSide, err = commitSideOrEmpty(r, rev); err != nil {
			return nil, err
		}
		newSide, err = indexSide(r)
	case len(opts.Revs) == 1:
		if oldSide, err = commitSide(r, opts.Revs[0]); err != nil {
			return nil, err
		}
		newSide, err = workSide(r, oldSide)
	default:
		if oldSide, err = indexSide(r); err != nil {
			return nil, err
		}
		newSide, err = workSide(r, oldSide)
	}
	if err != nil {
		return nil, err
	}
	return compare(oldSide, newSide, opts.Paths)
}

func compare(oldSide, newSide side, paths []string) ([]diff.File, error) {
	seen := map[string]bool{}
	var names []string
	for _, s := range []side{oldSide, newSide} {
		for p := range s {
			if !seen[p] && inPaths(p, paths) {
				seen[p] = true
				names = append(names, p)
			}
		}
	}
	sort.Strings(names)
	var out []diff.File
	for _, p := range names {
		o, hadOld := oldSide[p]
		n, hadNew := newSide[p]
		if hadOld && hadNew && o.id == n.id && o.mode == n.mode {
			continue
		}
		f := diff.File{Path: p}
		var err error
		if hadOld {
			f.OldMode, f.OldID = o.mode, o.id
			if f.Old, err = o.read(); err != nil {
				return nil, err
			}
		}
		if hadNew {
			f.NewMode, f.NewID = n.mode, n.id
			if f.New, err = n.read(); err != nil {
				return nil, err
			}
		}
		out = append(out, f)
	}
	return out, nil
}

func inPaths(p string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	for _, spec := range paths {
		if spec == "." || p == spec || strings.HasPrefix(p, strings.TrimSuffix(spec, "/")+"/") {
			return true
		}
	}
	return false
}

func blobReader(r *repo.Repo, id objects.ID) func() ([]byte, error) {
	return func() ([]byte, error) {
		b, err := r.ODB.ReadBlob(id)
		if err != nil {
			return nil, err
		}
		return b.Data, nil
	}
}

func commitSide(r *repo.Repo, rev string) (side, error) {
	id, err := revparse.ResolveCommit(r, rev)
	if err != nil {
		return nil, err
	}
	files, err := commitFiles(r, id)
	if err != nil {
		return nil, err
	}
	s := side{}
	for p, f := range files {
		s[p] = sideFile{mode: f.Mode, id: f.ID, read: blobReader(r, f.ID)}
	}
	return s, nil
}

// commitSideOrEmpty is commitSide, except that an unborn HEAD is an
// empty side. That lets `diff --staged` work before the first commit.
func commitSideOrEmpty(r *repo.Repo, rev string) (side, error) {
	if head, err := r.Refs.Head(); err == nil && head.Unborn() && rev == "HEAD" {
		return side{}, nil
	}
	return commitSide(r, rev)
}

func indexSide(r *repo.Repo) (side, error) {
	ix, err := r.LoadIndex()
	if err != nil {
		return nil, err
	}
	s := side{}
	for _, p := range ix.Paths() {
		if e, ok := ix.Get(p); ok {
			s[p] = sideFile{mode: e.Mode, id: e.ID, read: blobReader(r, e.ID)}
		}
	}
	return s, nil
}

// workSide reads the working tree for the paths the base side tracks.
// Untracked files are not part of a diff.
func workSide(r *repo.Repo, base side) (side, error) {
	s := side{}
	for p := range base {
		entry, data, err := readWorkFile(r, p)
		if isGone(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		content := data
		s[p] = sideFile{mode: entry.Mode, id: entry.ID, read: func() ([]byte, error) { return content, nil }}
	}
	return s, nil
}

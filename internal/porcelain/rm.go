package porcelain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/repo"
)

// RmOptions are the flags of `rm`.
type RmOptions struct {
	Cached    bool // --cached: only remove from the index
	Force     bool // -f: remove even with local modifications
	Recursive bool // -r: allow directories
}

// RmError says which paths stopped the command.
type RmError struct {
	Paths  []string
	Reason string
}

func (e *RmError) Error() string {
	return fmt.Sprintf("%s: %s", e.Reason, strings.Join(e.Paths, ", "))
}

// Rm removes tracked files from the index and, unless Cached is set,
// from the working tree. Without Force it refuses files that differ
// from both HEAD and the working tree, so no work is thrown away.
func Rm(r *repo.Repo, specs []string, opts RmOptions) ([]string, error) {
	if len(specs) == 0 {
		return nil, errors.New("nothing specified, nothing removed")
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return nil, err
	}
	committed, err := headFiles(r)
	if err != nil {
		return nil, err
	}

	var targets, dirs, modified, staged []string
	seen := map[string]bool{}
	for _, spec := range specs {
		spec = cleanSpec(spec)
		matched := false
		for _, p := range ix.Paths() {
			if p != spec && !strings.HasPrefix(p, spec+"/") {
				continue
			}
			matched = true
			if p != spec && !opts.Recursive {
				dirs = append(dirs, spec)
				break
			}
			if !seen[p] {
				seen[p] = true
				targets = append(targets, p)
			}
		}
		if !matched {
			return nil, &PathspecError{Spec: spec}
		}
	}
	if len(dirs) > 0 {
		return nil, &RmError{Paths: dirs, Reason: "not removing a directory without -r"}
	}
	sort.Strings(targets)

	if !opts.Force {
		for _, p := range targets {
			entry, ok := ix.Get(p)
			if !ok {
				return nil, &RmError{Paths: []string{p}, Reason: "path has unresolved conflicts, resolve them first"}
			}
			old, inHead := committed[p]
			stagedChange := !inHead || old.ID != entry.ID || old.Mode != entry.Mode
			disk, err := hashWorkFile(r, p)
			onDisk := err == nil
			if err != nil && !isGone(err) {
				return nil, err
			}
			unstagedChange := onDisk && (disk.ID != entry.ID || disk.Mode != entry.Mode)
			switch {
			case opts.Cached:
				if stagedChange && unstagedChange {
					modified = append(modified, p)
				}
			case unstagedChange:
				modified = append(modified, p)
			case stagedChange:
				staged = append(staged, p)
			}
		}
		if len(modified) > 0 {
			return nil, &RmError{Paths: modified, Reason: "files have local modifications, use -f to force removal"}
		}
		if len(staged) > 0 {
			return nil, &RmError{Paths: staged, Reason: "files have staged changes, use --cached to keep the file or -f to force removal"}
		}
	}

	for _, p := range targets {
		ix.Remove(p)
		if opts.Cached {
			continue
		}
		if err := r.Work.Remove(p); err != nil && !isGone(err) {
			return nil, err
		}
	}
	return targets, r.SaveIndex(ix)
}

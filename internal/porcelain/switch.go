package porcelain

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// ErrUnmergedIndex is returned when the index still has conflicts.
var ErrUnmergedIndex = errors.New("you need to resolve your current index first")

// LocalChangesError lists paths that the operation would overwrite.
type LocalChangesError struct {
	Paths     []string
	Untracked bool // the paths are untracked files, not modified ones
}

func (e *LocalChangesError) Error() string {
	what := "local changes to the following files would be overwritten"
	if e.Untracked {
		what = "untracked files would be overwritten"
	}
	return fmt.Sprintf("your %s: %s", what, strings.Join(e.Paths, ", "))
}

// SwitchOptions are the flags of `switch` and `checkout`.
type SwitchOptions struct {
	Create     bool   // -c / -b: create the branch first
	StartPoint string // where the new branch starts, default HEAD
	Detach     bool   // --detach
	Lenient    bool   // checkout style: a revision that is not a branch detaches HEAD
}

// SwitchResult describes where HEAD ended up.
type SwitchResult struct {
	FromBranch string // empty when HEAD was detached
	FromID     objects.ID
	ToBranch   string // empty when HEAD is now detached
	ToID       objects.ID
	Created    bool
	AlreadyOn  bool
	Detached   bool
	Files      int // files written or removed in the working tree
}

// Switch moves HEAD to a branch (or, with Detach or Lenient, to any
// commit) and updates the index and the working tree to match.
func Switch(r *repo.Repo, target string, opts SwitchOptions) (SwitchResult, error) {
	head, err := r.Refs.Head()
	if err != nil {
		return SwitchResult{}, err
	}
	res := SwitchResult{FromBranch: head.Branch(), FromID: head.ID}

	if opts.Create {
		return createAndSwitch(r, head, target, opts)
	}
	ref := refs.BranchPrefix + target
	branchID, err := r.Refs.Read(ref)
	isBranch := err == nil
	if err != nil && !errors.Is(err, refs.ErrNotFound) {
		return res, err
	}
	switch {
	case isBranch && !opts.Detach:
		if head.Ref == ref {
			res.AlreadyOn, res.ToBranch, res.ToID = true, target, branchID
			return res, nil
		}
		res.ToBranch, res.ToID = target, branchID
	case isBranch || opts.Detach || opts.Lenient:
		id, err := revparse.ResolveCommit(r, target)
		if err != nil {
			return res, err
		}
		res.ToID, res.Detached = id, true
	default:
		return res, fmt.Errorf("%w: %s", ErrBranchNotFound, target)
	}
	return res, moveToCommit(r, head, &res)
}

func createAndSwitch(r *repo.Repo, head refs.Head, name string, opts SwitchOptions) (SwitchResult, error) {
	res := SwitchResult{FromBranch: head.Branch(), FromID: head.ID, ToBranch: name, Created: true}
	if head.Unborn() && opts.StartPoint == "" {
		if err := refs.ValidName(refs.BranchPrefix + name); err != nil {
			return res, err
		}
		return res, r.Refs.SetHeadBranch(name)
	}
	id, err := CreateBranch(r, name, opts.StartPoint)
	if err != nil {
		return res, err
	}
	res.ToID = id
	return res, moveToCommit(r, head, &res)
}

// updateWorktree makes the index and the working tree match the target
// commit, starting from HEAD's tree, and returns how many paths changed.
// It refuses to run over uncommitted work (see checkLocalChanges).
func updateWorktree(r *repo.Repo, target objects.ID) (int, error) {
	cur, err := headFiles(r)
	if err != nil {
		return 0, err
	}
	tgt, err := commitFiles(r, target)
	if err != nil {
		return 0, err
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return 0, err
	}
	if len(ix.Conflicts()) > 0 {
		return 0, ErrUnmergedIndex
	}
	changed := differingPaths(cur, tgt)
	if err := checkLocalChanges(r, ix, cur, tgt, changed); err != nil {
		return 0, err
	}
	if err := syncPaths(r, ix, tgt, changed); err != nil {
		return 0, err
	}
	return len(changed), r.SaveIndex(ix)
}

// moveToCommit updates files, index and HEAD for res.ToID.
func moveToCommit(r *repo.Repo, from refs.Head, res *SwitchResult) error {
	files, err := updateWorktree(r, res.ToID)
	if err != nil {
		return err
	}
	res.Files = files

	who, err := r.Identity("committer")
	if err != nil {
		return err
	}
	if res.ToBranch != "" {
		err = r.Refs.SetHeadBranch(res.ToBranch)
	} else {
		err = r.Refs.SetHeadDetached(res.ToID)
	}
	if err != nil {
		return err
	}
	return r.Refs.AppendLog("HEAD", refs.LogEntry{
		Old: from.ID, New: res.ToID, Who: who,
		Message: "checkout: moving from " + describeHead(from) + " to " + describeTarget(res),
	})
}

func describeHead(h refs.Head) string {
	if h.Detached() {
		return h.ID.Short()
	}
	return h.Branch()
}

func describeTarget(res *SwitchResult) string {
	if res.ToBranch != "" {
		return res.ToBranch
	}
	return res.ToID.Short()
}

// differingPaths returns the sorted paths whose id or mode differ.
func differingPaths(a, b map[string]tree.File) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]tree.File{a, b} {
		for p := range m {
			if seen[p] {
				continue
			}
			seen[p] = true
			x, inA := a[p]
			y, inB := b[p]
			if inA != inB || x.ID != y.ID || x.Mode != y.Mode {
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// checkLocalChanges applies git's safety rule: a path that has to change
// must not carry uncommitted work or an untracked file.
func checkLocalChanges(r *repo.Repo, ix *index.Index, cur, tgt map[string]tree.File, paths []string) error {
	var modified, untracked []string
	for _, p := range paths {
		entry, tracked := ix.Get(p)
		old, inHead := cur[p]
		want, inTarget := tgt[p]
		onDisk := r.Work.Exists(p)

		if !tracked && !inHead {
			if onDisk {
				untracked = append(untracked, p)
			}
			continue
		}
		stagedSameAsTarget := tracked && inTarget && entry.ID == want.ID && entry.Mode == want.Mode
		stagedSameAsHead := tracked == inHead && (!tracked || (entry.ID == old.ID && entry.Mode == old.Mode))
		if !stagedSameAsHead && !stagedSameAsTarget {
			modified = append(modified, p)
			continue
		}
		if tracked && onDisk {
			disk, err := hashWorkFile(r, p)
			if err != nil {
				return err
			}
			dirty := disk.ID != entry.ID || disk.Mode != entry.Mode
			matchesTarget := inTarget && disk.ID == want.ID && disk.Mode == want.Mode
			if dirty && !matchesTarget {
				modified = append(modified, p)
			}
		} else if tracked && !onDisk {
			modified = append(modified, p)
		}
	}
	if len(modified) > 0 {
		return &LocalChangesError{Paths: modified}
	}
	if len(untracked) > 0 {
		return &LocalChangesError{Paths: untracked, Untracked: true}
	}
	return nil
}

// syncPaths makes the working tree and index match tgt for the given
// paths: files in tgt are written, files not in tgt are removed.
// Removals run first so that a file can replace a directory.
func syncPaths(r *repo.Repo, ix *index.Index, tgt map[string]tree.File, paths []string) error {
	for _, p := range paths {
		if _, ok := tgt[p]; ok {
			continue
		}
		ix.Remove(p)
		if err := r.Work.Remove(p); err != nil && !isGone(err) {
			return err
		}
	}
	for _, p := range paths {
		f, ok := tgt[p]
		if !ok {
			continue
		}
		blob, err := r.ODB.ReadBlob(f.ID)
		if err != nil {
			return fmt.Errorf("restoring %s: %w", p, err)
		}
		perm := fs.FileMode(0o644)
		if f.Mode == objects.ModeExecutable {
			perm = 0o755
		}
		if err := r.Work.WriteFile(p, blob.Data, perm); err != nil {
			return fmt.Errorf("restoring %s: %w", p, err)
		}
		info, err := r.Work.Stat(p)
		if err != nil {
			return err
		}
		ix.Add(index.Entry{Path: p, Mode: f.Mode, ID: f.ID, Size: uint32(len(blob.Data)), MTime: info.ModTime()})
	}
	return nil
}

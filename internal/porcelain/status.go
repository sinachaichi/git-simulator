package porcelain

import (
	"sort"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// ChangeKind says how a path differs.
type ChangeKind string

// The kinds of change status reports.
const (
	Added    ChangeKind = "new file"
	Modified ChangeKind = "modified"
	Deleted  ChangeKind = "deleted"
)

// Change is one path that differs between two states.
type Change struct {
	Path string
	Kind ChangeKind
}

// ConflictKind describes an unmerged path by which sides have it.
type ConflictKind string

// Conflict descriptions, as git's status prints them.
const (
	BothModified   ConflictKind = "both modified"
	BothAdded      ConflictKind = "both added"
	DeletedByUs    ConflictKind = "deleted by us"
	DeletedByThem  ConflictKind = "deleted by them"
	BothDeleted    ConflictKind = "both deleted"
	AddedByUs      ConflictKind = "added by us"
	AddedByThem    ConflictKind = "added by them"
	unknownConflct ConflictKind = "unmerged"
)

// Unmerged is a path with conflict stages in the index.
type Unmerged struct {
	Path string
	Kind ConflictKind
}

// StatusResult is everything `status` prints.
type StatusResult struct {
	Branch    string // empty when detached
	Detached  bool
	HeadID    objects.ID // zero on an unborn branch
	Unborn    bool
	Staged    []Change // HEAD vs index
	Unstaged  []Change // index vs working tree
	Untracked []string
	Unmerged  []Unmerged
}

// Clean reports whether there is nothing to show.
func (s StatusResult) Clean() bool {
	return len(s.Staged) == 0 && len(s.Unstaged) == 0 && len(s.Untracked) == 0 && len(s.Unmerged) == 0
}

// Status compares HEAD, the index and the working tree.
func Status(r *repo.Repo) (StatusResult, error) {
	head, err := r.Refs.Head()
	if err != nil {
		return StatusResult{}, err
	}
	res := StatusResult{Branch: head.Branch(), Detached: head.Detached(), HeadID: head.ID, Unborn: head.Unborn()}
	committed, err := headFiles(r)
	if err != nil {
		return res, err
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return res, err
	}
	work, err := r.Work.List()
	if err != nil {
		return res, err
	}
	onDisk := make(map[string]bool, len(work))
	for _, p := range work {
		onDisk[p] = true
	}

	conflicted := map[string]bool{}
	for _, p := range ix.Conflicts() {
		conflicted[p] = true
		res.Unmerged = append(res.Unmerged, Unmerged{Path: p, Kind: conflictKind(ix.Stages(p))})
	}

	tracked := map[string]bool{}
	for _, p := range ix.Paths() {
		if conflicted[p] {
			continue
		}
		tracked[p] = true
		e, _ := ix.Get(p)
		if old, ok := committed[p]; !ok {
			res.Staged = append(res.Staged, Change{p, Added})
		} else if old.ID != e.ID || old.Mode != e.Mode {
			res.Staged = append(res.Staged, Change{p, Modified})
		}
		if !onDisk[p] {
			res.Unstaged = append(res.Unstaged, Change{p, Deleted})
			continue
		}
		entry, err := hashWorkFile(r, p)
		if err != nil {
			return res, err
		}
		if entry.ID != e.ID || entry.Mode != e.Mode {
			res.Unstaged = append(res.Unstaged, Change{p, Modified})
		}
	}
	for p := range committed {
		if !tracked[p] && !conflicted[p] {
			res.Staged = append(res.Staged, Change{p, Deleted})
		}
	}
	sort.Slice(res.Staged, func(i, j int) bool { return res.Staged[i].Path < res.Staged[j].Path })
	for _, p := range work {
		if !tracked[p] && !conflicted[p] {
			res.Untracked = append(res.Untracked, p)
		}
	}
	return res, nil
}

func conflictKind(stages [3]*index.Entry) ConflictKind {
	base, ours, theirs := stages[0] != nil, stages[1] != nil, stages[2] != nil
	switch {
	case ours && theirs && base:
		return BothModified
	case ours && theirs:
		return BothAdded
	case base && theirs:
		return DeletedByUs
	case base && ours:
		return DeletedByThem
	case ours:
		return AddedByUs
	case theirs:
		return AddedByThem
	case base:
		return BothDeleted
	}
	return unknownConflct
}

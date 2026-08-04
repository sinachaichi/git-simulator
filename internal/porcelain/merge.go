package porcelain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// Errors returned by Merge.
var (
	ErrMergeInProgress   = errors.New("a merge is already in progress")
	ErrNoMergeInProgress = errors.New("there is no merge in progress")
	ErrUnrelated         = errors.New("refusing to merge unrelated histories")
	ErrNotFastForward    = errors.New("not possible to fast-forward")
	ErrNothingToMerge    = errors.New("HEAD has no commits to merge into")
)

const (
	mergeHeadFile = "MERGE_HEAD"
	mergeMsgFile  = "MERGE_MSG"
)

// MergeOptions are the flags of `merge`.
type MergeOptions struct {
	NoFF     bool   // --no-ff: always create a merge commit
	FFOnly   bool   // --ff-only: fail unless a fast-forward is possible
	NoCommit bool   // --no-commit: stop before creating the merge commit
	Message  string // -m: message of the merge commit
}

// MergeOutcome says what a merge did.
type MergeOutcome string

// The possible outcomes.
const (
	MergeUpToDate    MergeOutcome = "up to date"
	MergeFastForward MergeOutcome = "fast-forward"
	MergeCommitted   MergeOutcome = "merged"
	MergeStopped     MergeOutcome = "stopped before commit" // --no-commit
	MergeConflicted  MergeOutcome = "conflicted"
)

// ConflictInfo describes one conflicted path.
type ConflictInfo struct {
	Path string
	Kind merge.ConflictKind
}

// MergeResult is what Merge returns.
type MergeResult struct {
	Outcome   MergeOutcome
	From      objects.ID // HEAD before the merge
	Theirs    objects.ID // the commit that was merged in
	Commit    objects.ID // the new HEAD, for FastForward and Committed
	Base      objects.ID // merge base, zero for fast-forwards
	Files     int        // paths changed in the working tree
	Conflicts []ConflictInfo
}

// MergeInProgress reports whether a merge waits for a commit.
func MergeInProgress(r *repo.Repo) bool {
	_, err := os.Stat(filepath.Join(r.GitDir, mergeHeadFile))
	return err == nil
}

// Merge joins the history of rev into the current branch. It fast-forwards
// when HEAD is an ancestor of rev, and otherwise merges the trees against
// their common base. Conflicts leave the repo in a merge state: the index
// holds stages 1 to 3, the files hold markers, and MERGE_HEAD is set.
func Merge(r *repo.Repo, rev string, opts MergeOptions) (MergeResult, error) {
	var res MergeResult
	if MergeInProgress(r) {
		return res, ErrMergeInProgress
	}
	head, err := r.Refs.Head()
	if err != nil {
		return res, err
	}
	if head.Unborn() {
		return res, ErrNothingToMerge
	}
	theirs, err := revparse.ResolveCommit(r, rev)
	if err != nil {
		return res, err
	}
	res.From, res.Theirs = head.ID, theirs

	if ok, err := history.IsAncestor(r.ODB, theirs, head.ID); err != nil {
		return res, err
	} else if ok {
		res.Outcome = MergeUpToDate
		return res, nil
	}
	canFF, err := history.IsAncestor(r.ODB, head.ID, theirs)
	if err != nil {
		return res, err
	}
	if opts.FFOnly && !canFF {
		return res, ErrNotFastForward
	}
	if canFF && !opts.NoFF {
		return fastForward(r, rev, res)
	}
	return threeWay(r, head, rev, opts, res)
}

func fastForward(r *repo.Repo, rev string, res MergeResult) (MergeResult, error) {
	files, err := updateWorktree(r, res.Theirs)
	if err != nil {
		return res, err
	}
	if err := moveHead(r, res.Theirs, "merge "+rev+": Fast-forward"); err != nil {
		return res, err
	}
	res.Outcome, res.Commit, res.Files = MergeFastForward, res.Theirs, files
	return res, nil
}

func threeWay(r *repo.Repo, head refs.Head, rev string, opts MergeOptions, res MergeResult) (MergeResult, error) {
	bases, err := history.MergeBases(r.ODB, head.ID, res.Theirs)
	if err != nil {
		return res, err
	}
	if len(bases) == 0 {
		return res, ErrUnrelated
	}
	res.Base = bases[0] // for criss-cross histories git merges the bases first, we use the newest

	ix, err := r.LoadIndex()
	if err != nil {
		return res, err
	}
	ours, err := commitFiles(r, head.ID)
	if err != nil {
		return res, err
	}
	if staged := stagedPaths(ix, ours); len(staged) > 0 {
		return res, &LocalChangesError{Paths: staged}
	}
	baseFiles, err := flatFiles(r, res.Base)
	if err != nil {
		return res, err
	}
	theirFiles, err := flatFiles(r, res.Theirs)
	if err != nil {
		return res, err
	}
	merged, err := merge.Trees(r.ODB, baseFiles, filesOf(ours), theirFiles, merge.Labels{Ours: "HEAD", Theirs: rev})
	if err != nil {
		return res, err
	}

	mergedMap := make(map[string]tree.File, len(merged.Files))
	for _, f := range merged.Files {
		mergedMap[f.Path] = f
	}
	changed := differingPaths(ours, mergedMap)
	check := map[string]tree.File{}
	for p, f := range mergedMap {
		check[p] = f
	}
	for _, c := range merged.Conflicts {
		check[c.Path] = tree.File{Path: c.Path, Mode: c.Mode}
		changed = appendMissing(changed, c.Path)
	}
	sort.Strings(changed)
	if err := checkLocalChanges(r, ix, ours, check, changed); err != nil {
		return res, err
	}

	// Everything is checked, now write. Conflicted paths are handled
	// after the clean ones, so their markers are not overwritten.
	if err := syncPaths(r, ix, mergedMap, changed); err != nil {
		return res, err
	}
	for _, c := range merged.Conflicts {
		if err := writeConflict(r, ix, c); err != nil {
			return res, err
		}
		res.Conflicts = append(res.Conflicts, ConflictInfo{Path: c.Path, Kind: c.Kind})
	}
	if err := r.SaveIndex(ix); err != nil {
		return res, err
	}
	res.Files = len(changed)

	message := opts.Message
	if message == "" {
		message = defaultMergeMessage(r, head, rev)
	}
	if err := writeState(r, origHeadFile, head.ID.String()+"\n"); err != nil {
		return res, err
	}
	if err := writeState(r, mergeHeadFile, res.Theirs.String()+"\n"); err != nil {
		return res, err
	}
	if err := writeState(r, mergeMsgFile, CleanMessage(message)); err != nil {
		return res, err
	}
	switch {
	case len(res.Conflicts) > 0:
		res.Outcome = MergeConflicted
	case opts.NoCommit:
		res.Outcome = MergeStopped
	default:
		commit, err := Commit(r, CommitOptions{reflogMessage: "merge " + rev + ": Merge made by the 'ort' strategy."})
		if err != nil {
			return res, err
		}
		res.Outcome, res.Commit = MergeCommitted, commit.ID
	}
	return res, nil
}

// writeConflict puts the conflict markers in the working tree and the
// three stages in the index.
func writeConflict(r *repo.Repo, ix *index.Index, c merge.Conflict) error {
	perm := fs.FileMode(0o644)
	if c.Mode == objects.ModeExecutable {
		perm = 0o755
	}
	if err := r.Work.WriteFile(c.Path, c.Content, perm); err != nil {
		return err
	}
	ix.SetConflict(c.Path, entryOf(c.Base), entryOf(c.Ours), entryOf(c.Theirs))
	return nil
}

func entryOf(f *tree.File) *index.Entry {
	if f == nil {
		return nil
	}
	return &index.Entry{Path: f.Path, Mode: f.Mode, ID: f.ID}
}

// stagedPaths lists paths where the index differs from the commit.
func stagedPaths(ix *index.Index, committed map[string]tree.File) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range ix.Paths() {
		seen[p] = true
		e, ok := ix.Get(p)
		old, inHead := committed[p]
		if !ok || !inHead || e.ID != old.ID || e.Mode != old.Mode {
			out = append(out, p)
		}
	}
	for p := range committed {
		if !seen[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func flatFiles(r *repo.Repo, id objects.ID) ([]tree.File, error) {
	c, err := r.ODB.ReadCommit(id)
	if err != nil {
		return nil, err
	}
	return tree.Flatten(r.ODB, c.Tree)
}

func filesOf(m map[string]tree.File) []tree.File {
	out := make([]tree.File, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func appendMissing(list []string, p string) []string {
	for _, x := range list {
		if x == p {
			return list
		}
	}
	return append(list, p)
}

// defaultMergeMessage follows git: "Merge branch 'x'", "Merge tag 'x'" or
// "Merge commit 'x'", with " into <branch>" unless that branch is main
// or master.
func defaultMergeMessage(r *repo.Repo, head refs.Head, rev string) string {
	kind := "commit"
	switch {
	case r.Refs.Exists(refs.BranchPrefix + rev):
		kind = "branch"
	case r.Refs.Exists(refs.TagPrefix + rev):
		kind = "tag"
	}
	msg := fmt.Sprintf("Merge %s '%s'", kind, rev)
	if b := head.Branch(); b != "" && b != "main" && b != "master" {
		msg += " into " + b
	}
	return msg
}

// readMergeState returns the pending merge parent and message, if any.
func readMergeState(r *repo.Repo) (parent objects.ID, message string, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(r.GitDir, mergeHeadFile))
	if errors.Is(err, fs.ErrNotExist) {
		return objects.Zero, "", false, nil
	}
	if err != nil {
		return objects.Zero, "", false, err
	}
	parent, err = objects.ParseID(strings.TrimSpace(string(data)))
	if err != nil {
		return objects.Zero, "", false, fmt.Errorf("MERGE_HEAD is corrupt: %w", err)
	}
	msg, err := os.ReadFile(filepath.Join(r.GitDir, mergeMsgFile))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return objects.Zero, "", false, err
	}
	return parent, string(msg), true, nil
}

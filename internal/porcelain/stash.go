package porcelain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

const stashRef = "refs/stash"

// Errors returned by the stash functions.
var (
	ErrNoLocalChanges = errors.New("no local changes to save")
	ErrNoStash        = errors.New("no stash entry found")
)

// StashEntry is one line of `stash list`.
type StashEntry struct {
	Index   int
	ID      objects.ID
	Message string
}

// StashPush saves the index and the tracked files of the working tree and
// resets both to HEAD. It stores them like git: a "WIP" commit whose
// tree is the working tree and whose second parent is a commit of the
// index. Untracked files are not stashed.
func StashPush(r *repo.Repo, message string) (StashEntry, error) {
	head, err := r.Refs.Head()
	if err != nil {
		return StashEntry{}, err
	}
	if head.Unborn() {
		return StashEntry{}, errors.New("you do not have the initial commit yet")
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return StashEntry{}, err
	}
	indexTree, err := ix.WriteTree(r.ODB)
	if err != nil {
		return StashEntry{}, err
	}
	workTree, err := workingTreeOfTracked(r, ix)
	if err != nil {
		return StashEntry{}, err
	}
	headCommit, err := r.ODB.ReadCommit(head.ID)
	if err != nil {
		return StashEntry{}, err
	}
	if indexTree == headCommit.Tree && workTree == headCommit.Tree {
		return StashEntry{}, ErrNoLocalChanges
	}

	who, err := r.Identity("committer")
	if err != nil {
		return StashEntry{}, err
	}
	summary, err := headSummary(r, head)
	if err != nil {
		return StashEntry{}, err
	}
	branch := head.Branch()
	if branch == "" {
		branch = "(no branch)"
	}
	indexCommit, _, err := r.ODB.Write(&objects.Commit{
		Tree: indexTree, Parents: []objects.ID{head.ID}, Author: who, Committer: who,
		Message: "index on " + branch + ": " + summary + "\n",
	})
	if err != nil {
		return StashEntry{}, err
	}
	subject := "WIP on " + branch + ": " + summary
	if m := strings.TrimSpace(message); m != "" {
		subject = "On " + branch + ": " + m
	}
	stashCommit, _, err := r.ODB.Write(&objects.Commit{
		Tree: workTree, Parents: []objects.ID{head.ID, indexCommit}, Author: who, Committer: who,
		Message: subject + "\n",
	})
	if err != nil {
		return StashEntry{}, err
	}

	old, err := r.Refs.Read(stashRef)
	if err != nil && !errors.Is(err, refs.ErrNotFound) {
		return StashEntry{}, err
	}
	if err := r.Refs.Update(stashRef, stashCommit); err != nil {
		return StashEntry{}, err
	}
	if err := r.Refs.AppendLog(stashRef, refs.LogEntry{Old: old, New: stashCommit, Who: who, Message: subject}); err != nil {
		return StashEntry{}, err
	}
	// Both the index and the files go back to HEAD. ORIG_HEAD is left alone.
	if err := resetIndexAndFiles(r, head.ID, true); err != nil {
		return StashEntry{}, err
	}
	return StashEntry{Index: 0, ID: stashCommit, Message: subject}, nil
}

// workingTreeOfTracked writes a tree with the current content of every
// file the index tracks. Files deleted on disk are left out.
func workingTreeOfTracked(r *repo.Repo, ix *index.Index) (objects.ID, error) {
	var files []tree.File
	for _, p := range ix.Paths() {
		entry, err := stageFile(r, p)
		if isGone(err) {
			continue
		}
		if err != nil {
			return objects.Zero, err
		}
		files = append(files, tree.File{Path: p, Mode: entry.Mode, ID: entry.ID})
	}
	return tree.Build(r.ODB, files)
}

func headSummary(r *repo.Repo, head refs.Head) (string, error) {
	c, err := r.ODB.ReadCommit(head.ID)
	if err != nil {
		return "", err
	}
	short, err := r.ODB.Abbrev(head.ID, 7)
	if err != nil {
		return "", err
	}
	return short + " " + c.Subject(), nil
}

// StashList returns the stack, newest entry first.
func StashList(r *repo.Repo) ([]StashEntry, error) {
	log, err := r.Refs.ReadLog(stashRef)
	if err != nil {
		return nil, err
	}
	out := make([]StashEntry, len(log))
	for i, e := range log {
		out[i] = StashEntry{Index: i, ID: e.New, Message: e.Message}
	}
	return out, nil
}

func stashAt(r *repo.Repo, n int) (StashEntry, error) {
	list, err := StashList(r)
	if err != nil {
		return StashEntry{}, err
	}
	if n < 0 || n >= len(list) {
		return StashEntry{}, fmt.Errorf("%w: stash@{%d}", ErrNoStash, n)
	}
	return list[n], nil
}

// StashApply replays stash@{n} on top of the current HEAD with a three-way
// merge: the stash's parent is the base, HEAD is ours, the stash is
// theirs. Changes come back unstaged, except new files, which stay
// staged as in git. The stash entry is kept.
func StashApply(r *repo.Repo, n int) ([]ConflictInfo, error) {
	entry, err := stashAt(r, n)
	if err != nil {
		return nil, err
	}
	stash, err := r.ODB.ReadCommit(entry.ID)
	if err != nil {
		return nil, err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return nil, err
	}
	baseFiles, err := flatFiles(r, stash.Parents[0])
	if err != nil {
		return nil, err
	}
	stashFiles, err := tree.Flatten(r.ODB, stash.Tree)
	if err != nil {
		return nil, err
	}
	conflicts, _, err := applyMerge(r, head, baseFiles, stashFiles,
		merge.Labels{Ours: "Updated upstream", Theirs: "Stashed changes"})
	if err != nil {
		return nil, err
	}
	if len(conflicts) == 0 {
		return nil, unstageModifications(r)
	}
	return conflicts, nil
}

// unstageModifications points the index entries of files that HEAD
// already has back at HEAD, keeping the working tree as it is.
func unstageModifications(r *repo.Repo) error {
	committed, err := headFiles(r)
	if err != nil {
		return err
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return err
	}
	for _, p := range ix.Paths() {
		if old, ok := committed[p]; ok {
			ix.Add(index.Entry{Path: p, Mode: old.Mode, ID: old.ID})
		}
	}
	for p, f := range committed {
		if _, ok := ix.Get(p); !ok {
			ix.Add(index.Entry{Path: p, Mode: f.Mode, ID: f.ID})
		}
	}
	return r.SaveIndex(ix)
}

// StashDrop removes stash@{n} and returns the entry that was dropped.
func StashDrop(r *repo.Repo, n int) (StashEntry, error) {
	entry, err := stashAt(r, n)
	if err != nil {
		return StashEntry{}, err
	}
	log, err := r.Refs.ReadLog(stashRef)
	if err != nil {
		return StashEntry{}, err
	}
	log = append(log[:n:n], log[n+1:]...)
	if err := r.Refs.ReplaceLog(stashRef, log); err != nil {
		return StashEntry{}, err
	}
	if len(log) == 0 {
		return entry, r.Refs.Delete(stashRef)
	}
	return entry, r.Refs.Update(stashRef, log[0].New)
}

// StashPop applies stash@{n} and drops it when there are no conflicts.
func StashPop(r *repo.Repo, n int) ([]ConflictInfo, StashEntry, error) {
	conflicts, err := StashApply(r, n)
	if err != nil || len(conflicts) > 0 {
		return conflicts, StashEntry{}, err
	}
	dropped, err := StashDrop(r, n)
	return nil, dropped, err
}

// StashShow returns the changes saved in stash@{n}, against the commit
// it was made on.
func StashShow(r *repo.Repo, n int) ([]diff.File, error) {
	entry, err := stashAt(r, n)
	if err != nil {
		return nil, err
	}
	stash, err := r.ODB.ReadCommit(entry.ID)
	if err != nil {
		return nil, err
	}
	base, err := r.ODB.ReadCommit(stash.Parents[0])
	if err != nil {
		return nil, err
	}
	oldSide, err := treeSide(r, base.Tree)
	if err != nil {
		return nil, err
	}
	newSide, err := treeSide(r, stash.Tree)
	if err != nil {
		return nil, err
	}
	return compare(oldSide, newSide, nil)
}

package porcelain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// Errors returned by CherryPick and Revert.
var (
	ErrPickInProgress = errors.New("a cherry-pick or revert is already in progress")
	ErrNoPick         = errors.New("there is no cherry-pick or revert in progress")
	ErrMergeCommit    = errors.New("is a merge commit, picking or reverting merges is not supported")
	ErrEmptyPick      = errors.New("the change is already in the current branch, nothing to commit")
)

const (
	cherryPickHeadFile = "CHERRY_PICK_HEAD"
	revertHeadFile     = "REVERT_HEAD"
)

// PickOptions are the flags of `cherry-pick` and `revert`.
type PickOptions struct {
	NoCommit bool // -n: leave the result staged
}

// PickResult describes a finished or stopped cherry-pick or revert.
type PickResult struct {
	Outcome   MergeOutcome // MergeCommitted, MergeStopped or MergeConflicted
	Source    objects.ID   // the commit that was picked or reverted
	Commit    objects.ID   // the new commit, when one was made
	Files     int
	Conflicts []ConflictInfo
}

// CherryPick applies the change made by rev on top of HEAD and commits it
// with the original author and message.
func CherryPick(r *repo.Repo, rev string, opts PickOptions) (PickResult, error) {
	return pick(r, rev, opts, false)
}

// Revert applies the opposite of the change made by rev and commits it.
func Revert(r *repo.Repo, rev string, opts PickOptions) (PickResult, error) {
	return pick(r, rev, opts, true)
}

func pick(r *repo.Repo, rev string, opts PickOptions, reverse bool) (PickResult, error) {
	var res PickResult
	if PickInProgress(r) || MergeInProgress(r) {
		return res, ErrPickInProgress
	}
	head, err := r.Refs.Head()
	if err != nil {
		return res, err
	}
	if head.Unborn() {
		return res, ErrNothingToMerge
	}
	id, err := revparse.ResolveCommit(r, rev)
	if err != nil {
		return res, err
	}
	res.Source = id
	c, err := r.ODB.ReadCommit(id)
	if err != nil {
		return res, err
	}
	if len(c.Parents) > 1 {
		return res, fmt.Errorf("commit %s %w", id.Short(), ErrMergeCommit)
	}

	// The change of a commit is the step from its parent to itself.
	var parentFiles []tree.File
	if len(c.Parents) == 1 {
		if parentFiles, err = flatFiles(r, c.Parents[0]); err != nil {
			return res, err
		}
	}
	commitFilesList, err := flatFiles(r, id)
	if err != nil {
		return res, err
	}
	base, theirs := parentFiles, commitFilesList
	label := id.Short() + " (" + c.Subject() + ")"
	message, stateFile, verb := c.Message, cherryPickHeadFile, "cherry-pick"
	if reverse {
		base, theirs = commitFilesList, parentFiles
		label = "parent of " + label
		message = fmt.Sprintf("Revert \"%s\"\n\nThis reverts commit %s.\n", c.Subject(), id)
		stateFile, verb = revertHeadFile, "revert"
	}

	conflicts, files, err := applyMerge(r, head, base, theirs, merge.Labels{Ours: "HEAD", Theirs: label})
	if err != nil {
		return res, err
	}
	res.Conflicts, res.Files = conflicts, files
	if err := writeState(r, origHeadFile, head.ID.String()+"\n"); err != nil {
		return res, err
	}
	if err := writeState(r, stateFile, id.String()+"\n"); err != nil {
		return res, err
	}
	if err := writeState(r, mergeMsgFile, CleanMessage(message)); err != nil {
		return res, err
	}
	switch {
	case len(conflicts) > 0:
		res.Outcome = MergeConflicted
	case opts.NoCommit:
		res.Outcome = MergeStopped
	default:
		commit, err := Commit(r, CommitOptions{reflogMessage: verb + ": " + c.Subject()})
		if errors.Is(err, ErrNothingToCommit) {
			clearMergeState(r)
			return res, ErrEmptyPick
		}
		if err != nil {
			return res, err
		}
		res.Outcome, res.Commit = MergeCommitted, commit.ID
	}
	return res, nil
}

// PickInProgress reports whether a cherry-pick or revert waits for a commit.
func PickInProgress(r *repo.Repo) bool {
	_, _, ok, _ := readPickState(r)
	return ok
}

// PickAbort gives up a stopped cherry-pick or revert and resets to HEAD.
func PickAbort(r *repo.Repo) error {
	if !PickInProgress(r) {
		return ErrNoPick
	}
	_, err := Reset(r, "HEAD", ResetHard)
	return err
}

// readPickState returns the commit being picked or reverted, and whether
// it is a revert.
func readPickState(r *repo.Repo) (id objects.ID, reverting, ok bool, err error) {
	for _, name := range []string{cherryPickHeadFile, revertHeadFile} {
		data, err := os.ReadFile(filepath.Join(r.GitDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return objects.Zero, false, false, err
		}
		id, err := objects.ParseID(strings.TrimSpace(string(data)))
		if err != nil {
			return objects.Zero, false, false, fmt.Errorf("%s is corrupt: %w", name, err)
		}
		return id, name == revertHeadFile, true, nil
	}
	return objects.Zero, false, false, nil
}

func isRevert(r *repo.Repo) bool {
	_, reverting, _, _ := readPickState(r)
	return reverting
}

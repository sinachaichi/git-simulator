package porcelain_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func TestCherryPickAppliesOneCommitWithOriginalAuthor(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "base")
	_, err := porcelain.Switch(r, "feature", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	write(t, r, "wanted", "w\n")
	t.Setenv("GIT_AUTHOR_NAME", "Grace Hopper")
	commitAll(t, r, "add wanted")
	picked := headID(t, r)
	write(t, r, "unwanted", "u\n")
	t.Setenv("GIT_AUTHOR_NAME", "Ada Lovelace")
	commitAll(t, r, "add unwanted")
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)

	tick(t)
	res, err := porcelain.CherryPick(r, "feature~1", porcelain.PickOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeCommitted, res.Outcome)
	assert.Equal(t, picked, res.Source)

	c := commitOf(t, r, res.Commit)
	assert.Equal(t, "add wanted\n", c.Message)
	assert.Equal(t, "Grace Hopper", c.Author.Name, "the author is kept")
	assert.Equal(t, "Ada Lovelace", c.Committer.Name, "the committer is whoever picks")
	assert.NotEqual(t, picked, res.Commit)
	assert.Equal(t, "w\n", read(t, r, "wanted"))
	assert.False(t, r.Work.Exists("unwanted"))
	log, _ := r.Refs.ReadLog("HEAD")
	assert.Equal(t, "cherry-pick: add wanted", log[0].Message)
}

func TestCherryPickNoCommitStagesTheChange(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	before := headID(t, r)
	res, err := porcelain.CherryPick(r, "feature", porcelain.PickOptions{NoCommit: true})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeStopped, res.Outcome)
	assert.Equal(t, before, headID(t, r))
	assert.NotEmpty(t, status(t, r).Staged)
}

func TestCherryPickConflictThenResolveAndCommit(t *testing.T) {
	r := conflicting(t)
	mainTip := headID(t, r)
	res, err := porcelain.CherryPick(r, "feature", porcelain.PickOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeConflicted, res.Outcome)
	assert.Equal(t, []porcelain.ConflictInfo{{Path: "shared.txt", Kind: merge.ContentConflict, HasOurs: true, HasTheirs: true}}, res.Conflicts)
	assert.True(t, porcelain.PickInProgress(r))
	assert.Equal(t, mainTip, headID(t, r))
	assert.Contains(t, read(t, r, "shared.txt"), ">>>>>>> ")

	_, err = porcelain.CherryPick(r, "feature", porcelain.PickOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrPickInProgress))

	write(t, r, "shared.txt", "resolved\n")
	_, err = porcelain.Add(r, []string{"shared.txt"})
	require.NoError(t, err)
	tick(t)
	done, err := porcelain.Commit(r, porcelain.CommitOptions{})
	require.NoError(t, err)
	assert.Equal(t, "feature work\n", commitOf(t, r, done.ID).Message)
	assert.Len(t, commitOf(t, r, done.ID).Parents, 1)
	assert.False(t, porcelain.PickInProgress(r))
}

func TestCherryPickRootCommitAndAbort(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "root")
	root := headID(t, r)
	_, err := porcelain.Switch(r, "other", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("a"))
	write(t, r, "a", "other\n")
	commitAll(t, r, "change a")

	// Picking the root commit adds "a" again, which conflicts with the edit.
	res, err := porcelain.CherryPick(r, root.String(), porcelain.PickOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeConflicted, res.Outcome)
	assert.Equal(t, merge.AddAdd, res.Conflicts[0].Kind)

	require.NoError(t, porcelain.PickAbort(r))
	assert.False(t, porcelain.PickInProgress(r))
	assert.Equal(t, "other\n", read(t, r, "a"))
	assert.True(t, errors.Is(porcelain.PickAbort(r), porcelain.ErrNoPick))
}

func TestCherryPickOfAChangeThatIsAlreadyThere(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	_, err = porcelain.CherryPick(r, "feature", porcelain.PickOptions{})
	require.NoError(t, err)
	tick(t)
	_, err = porcelain.CherryPick(r, "feature", porcelain.PickOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrEmptyPick))
	assert.False(t, porcelain.PickInProgress(r))
}

func TestCherryPickRefusesMergeCommits(t *testing.T) {
	r := disjointEdits(t)
	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	_, err = porcelain.CherryPick(r, res.Commit.String(), porcelain.PickOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrMergeCommit))
	_, err = porcelain.Revert(r, res.Commit.String(), porcelain.PickOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrMergeCommit))
}

func TestRevertUndoesACommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	write(t, r, "a", "2\n")
	write(t, r, "b", "new\n")
	commitAll(t, r, "two")
	bad := headID(t, r)
	write(t, r, "c", "later work\n")
	commitAll(t, r, "three")

	tick(t)
	res, err := porcelain.Revert(r, "HEAD~1", porcelain.PickOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeCommitted, res.Outcome)

	c := commitOf(t, r, res.Commit)
	assert.Equal(t, "Revert \"two\"\n\nThis reverts commit "+bad.String()+".\n", c.Message)
	assert.Equal(t, "1\n", read(t, r, "a"))
	assert.False(t, r.Work.Exists("b"))
	assert.Equal(t, "later work\n", read(t, r, "c"), "newer work stays")
	assert.Equal(t, "Ada Lovelace", c.Author.Name)
	log, _ := r.Refs.ReadLog("HEAD")
	assert.Equal(t, "revert: two", log[0].Message)
}

func TestRevertConflictAndNoCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	write(t, r, "a", "2\n")
	commitAll(t, r, "two")
	write(t, r, "a", "3\n")
	commitAll(t, r, "three")

	res, err := porcelain.Revert(r, "HEAD~1", porcelain.PickOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeConflicted, res.Outcome)
	assert.Contains(t, read(t, r, "a"), "parent of ")
	require.NoError(t, porcelain.PickAbort(r))

	res, err = porcelain.Revert(r, "HEAD", porcelain.PickOptions{NoCommit: true})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeStopped, res.Outcome)
	assert.Equal(t, "2\n", read(t, r, "a"))
	assert.Len(t, status(t, r).Staged, 1)
	tick(t)
	done, err := porcelain.Commit(r, porcelain.CommitOptions{})
	require.NoError(t, err)
	assert.Contains(t, commitOf(t, r, done.ID).Message, "Revert \"three\"")
}

func TestRevertWhileDirtyAndUnknownRevision(t *testing.T) {
	r := twoCommits(t)
	write(t, r, "f.txt", "dirty\n")
	_, err := porcelain.Revert(r, "HEAD", porcelain.PickOptions{})
	var lc *porcelain.LocalChangesError
	assert.True(t, errors.As(err, &lc))
	_, err = porcelain.Revert(r, "ghost", porcelain.PickOptions{})
	assert.Error(t, err)
	_, err = porcelain.CherryPick(newRepo(t), "x", porcelain.PickOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrNothingToMerge))
}

// Real git, run on a copy of the repo, produces the same trees.
func TestPickAndRevertMatchRealGit(t *testing.T) {
	testutil.RequireGit(t)
	r := diverged(t,
		func(r *repo.Repo) { write(t, r, "main-only", "m\n") },
		func(r *repo.Repo) {
			write(t, r, "shared.txt", numbered(10)+"feature line\n")
			write(t, r, "f", "f\n")
		})
	copyDir := t.TempDir()
	testutil.CopyDir(t, r.Root, copyDir)
	gitDir := filepath.Join(copyDir, ".hood")
	realGit := func(args ...string) string {
		return testutil.Git(t, copyDir, "", append([]string{"--git-dir=" + gitDir, "--work-tree=" + copyDir}, args...)...)
	}

	tick(t)
	picked, err := porcelain.CherryPick(r, "feature", porcelain.PickOptions{})
	require.NoError(t, err)
	realGit("cherry-pick", "feature")
	assert.Equal(t, commitOf(t, r, picked.Commit).Tree.String()+"\n", realGit("rev-parse", "HEAD^{tree}"))

	tick(t)
	reverted, err := porcelain.Revert(r, "HEAD", porcelain.PickOptions{})
	require.NoError(t, err)
	realGit("revert", "--no-edit", "HEAD")
	assert.Equal(t, commitOf(t, r, reverted.Commit).Tree.String()+"\n", realGit("rev-parse", "HEAD^{tree}"))
}

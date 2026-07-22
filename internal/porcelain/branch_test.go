package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func twoCommits(t *testing.T) *repo.Repo {
	t.Helper()
	return linearRepo(t, "first", "second")
}

func TestCreateAndListBranches(t *testing.T) {
	r := twoCommits(t)
	id, err := porcelain.CreateBranch(r, "feature", "")
	require.NoError(t, err)
	head, _ := r.Refs.Head()
	assert.Equal(t, head.ID, id)

	_, err = porcelain.CreateBranch(r, "old", "HEAD~1")
	require.NoError(t, err)

	list, err := porcelain.ListBranches(r)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{"feature", "main", "old"}, []string{list[0].Name, list[1].Name, list[2].Name})
	assert.True(t, list[1].Current)
	assert.Equal(t, "second", list[1].Subject)
	assert.Equal(t, "first", list[2].Subject)
}

func TestCreateBranchWritesReflog(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateBranch(r, "feature", "HEAD~1")
	require.NoError(t, err)
	log, err := r.Refs.ReadLog("refs/heads/feature")
	require.NoError(t, err)
	require.Len(t, log, 1)
	assert.Equal(t, "branch: Created from HEAD~1", log[0].Message)
}

func TestCreateBranchErrors(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateBranch(r, "main", "")
	assert.True(t, errors.Is(err, porcelain.ErrBranchExists))
	_, err = porcelain.CreateBranch(r, "bad name", "")
	assert.Error(t, err)
	_, err = porcelain.CreateBranch(r, "x", "nope")
	assert.Error(t, err)

	_, err = porcelain.CreateBranch(newRepo(t), "x", "")
	assert.ErrorContains(t, err, "no commits", "an unborn branch has nothing to branch from")
}

func TestDeleteMergedBranch(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateBranch(r, "done", "HEAD~1")
	require.NoError(t, err)
	_, err = porcelain.DeleteBranch(r, "done", false)
	require.NoError(t, err)
	assert.False(t, r.Refs.Exists("refs/heads/done"))
	log, _ := r.Refs.ReadLog("refs/heads/done")
	assert.Empty(t, log)
}

func TestDeleteRefusesUnmergedAndCurrent(t *testing.T) {
	r := twoCommits(t)
	head, _ := r.Refs.Head()
	// A branch with a commit that main does not have.
	require.NoError(t, r.Refs.Update("refs/heads/side", head.ID))
	require.NoError(t, r.Refs.SetHeadBranch("side"))
	write(t, r, "extra", "x")
	commitAll(t, r, "side work")
	require.NoError(t, r.Refs.SetHeadBranch("main"))

	_, err := porcelain.DeleteBranch(r, "side", false)
	assert.True(t, errors.Is(err, porcelain.ErrNotMerged))
	assert.True(t, r.Refs.Exists("refs/heads/side"))

	_, err = porcelain.DeleteBranch(r, "side", true)
	require.NoError(t, err)

	_, err = porcelain.DeleteBranch(r, "main", true)
	assert.True(t, errors.Is(err, porcelain.ErrCurrentBranch))
	_, err = porcelain.DeleteBranch(r, "ghost", false)
	assert.True(t, errors.Is(err, porcelain.ErrBranchNotFound))
}

func TestRenameBranch(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateBranch(r, "topic", "")
	require.NoError(t, err)
	require.NoError(t, porcelain.RenameBranch(r, "topic", "feat/topic"))
	assert.False(t, r.Refs.Exists("refs/heads/topic"))
	assert.True(t, r.Refs.Exists("refs/heads/feat/topic"))
	log, _ := r.Refs.ReadLog("refs/heads/feat/topic")
	assert.Len(t, log, 1, "the reflog moves with the branch")
}

func TestRenameCurrentBranchMovesHead(t *testing.T) {
	r := twoCommits(t)
	require.NoError(t, porcelain.RenameBranch(r, "main", "trunk"))
	head, err := r.Refs.Head()
	require.NoError(t, err)
	assert.Equal(t, "trunk", head.Branch())
	assert.False(t, head.Unborn())
}

func TestRenameUnbornBranch(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, porcelain.RenameBranch(r, "main", "trunk"))
	head, _ := r.Refs.Head()
	assert.Equal(t, "trunk", head.Branch())
}

func TestRenameErrors(t *testing.T) {
	r := twoCommits(t)
	_, _ = porcelain.CreateBranch(r, "other", "")
	assert.True(t, errors.Is(porcelain.RenameBranch(r, "ghost", "x"), porcelain.ErrBranchNotFound))
	assert.True(t, errors.Is(porcelain.RenameBranch(r, "other", "main"), porcelain.ErrBranchExists))
	assert.Error(t, porcelain.RenameBranch(r, "other", "bad name"))
}

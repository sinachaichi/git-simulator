package porcelain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// threeCommits makes a.txt hold v1, v2 and v3 in turn, and adds b.txt in
// the last commit.
func threeCommits(t *testing.T) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "a.txt", "v1\n")
	commitAll(t, r, "one")
	write(t, r, "a.txt", "v2\n")
	commitAll(t, r, "two")
	write(t, r, "a.txt", "v3\n")
	write(t, r, "b.txt", "b\n")
	commitAll(t, r, "three")
	return r
}

func headID(t *testing.T, r *repo.Repo) objects.ID {
	t.Helper()
	h, err := r.Refs.Head()
	require.NoError(t, err)
	return h.ID
}

func TestResetSoftKeepsIndexAndFiles(t *testing.T) {
	r := threeCommits(t)
	tip := headID(t, r)
	res, err := porcelain.Reset(r, "HEAD~2", porcelain.ResetSoft)
	require.NoError(t, err)
	assert.Equal(t, tip, res.From)
	assert.Equal(t, headID(t, r), res.To)

	assert.Equal(t, "v3\n", read(t, r, "a.txt"))
	s := status(t, r)
	assert.Len(t, s.Staged, 2, "the undone commits are now staged")
	assert.Empty(t, s.Unstaged)
}

func TestResetMixedResetsIndexOnly(t *testing.T) {
	r := threeCommits(t)
	_, err := porcelain.Reset(r, "HEAD~1", porcelain.ResetMixed)
	require.NoError(t, err)

	assert.Equal(t, "v3\n", read(t, r, "a.txt"))
	s := status(t, r)
	assert.Empty(t, s.Staged)
	assert.Equal(t, []porcelain.Change{{Path: "a.txt", Kind: porcelain.Modified}}, s.Unstaged)
	assert.Equal(t, []string{"b.txt"}, s.Untracked)
}

func TestResetHardResetsEverythingTracked(t *testing.T) {
	r := threeCommits(t)
	write(t, r, "untracked.txt", "keep me\n")
	write(t, r, "a.txt", "local edit\n")
	_, err := porcelain.Reset(r, "HEAD~1", porcelain.ResetHard)
	require.NoError(t, err)

	assert.Equal(t, "v2\n", read(t, r, "a.txt"))
	assert.False(t, r.Work.Exists("b.txt"))
	assert.Equal(t, "keep me\n", read(t, r, "untracked.txt"))
	s := status(t, r)
	assert.Empty(t, s.Staged)
	assert.Empty(t, s.Unstaged)
	assert.Equal(t, []string{"untracked.txt"}, s.Untracked)
}

func TestResetHardDiscardsEditsWhenTargetIsHead(t *testing.T) {
	r := threeCommits(t)
	write(t, r, "a.txt", "scribble\n")
	_, err := porcelain.Reset(r, "HEAD", porcelain.ResetHard)
	require.NoError(t, err)
	assert.Equal(t, "v3\n", read(t, r, "a.txt"))
}

func TestResetMovesOnlyTheCurrentBranch(t *testing.T) {
	r := threeCommits(t)
	tip := headID(t, r)
	_, err := porcelain.CreateBranch(r, "keep", "")
	require.NoError(t, err)
	_, err = porcelain.Reset(r, "HEAD~2", porcelain.ResetHard)
	require.NoError(t, err)
	keep, _ := r.Refs.Read("refs/heads/keep")
	assert.Equal(t, tip, keep)
}

func TestResetSavesOrigHeadAndLogsIt(t *testing.T) {
	r := threeCommits(t)
	tip := headID(t, r)
	_, err := porcelain.Reset(r, "HEAD~1", porcelain.ResetHard)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(r.GitDir, "ORIG_HEAD"))
	require.NoError(t, err)
	assert.Equal(t, tip.String()+"\n", string(data))

	log, err := r.Refs.ReadLog("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "reset: moving to HEAD~1", log[0].Message)
	assert.Equal(t, tip, log[0].Old)

	// ORIG_HEAD brings the commit back.
	_, err = porcelain.Reset(r, "ORIG_HEAD", porcelain.ResetHard)
	require.NoError(t, err)
	assert.Equal(t, tip, headID(t, r))
}

func TestResetOnDetachedHead(t *testing.T) {
	r := threeCommits(t)
	require.NoError(t, r.Refs.SetHeadDetached(headID(t, r)))
	_, err := porcelain.Reset(r, "HEAD~1", porcelain.ResetHard)
	require.NoError(t, err)
	h, _ := r.Refs.Head()
	assert.True(t, h.Detached())
	main, _ := r.Refs.Read("refs/heads/main")
	assert.NotEqual(t, h.ID, main, "the branch stays where it was")
}

func TestResetErrors(t *testing.T) {
	r := threeCommits(t)
	_, err := porcelain.Reset(r, "nope", porcelain.ResetHard)
	assert.Error(t, err)
	_, err = porcelain.Reset(r, "HEAD", "weird")
	assert.ErrorContains(t, err, "unknown reset mode")

	require.NoError(t, saveConflict(r, "a.txt"))
	_, err = porcelain.Reset(r, "HEAD~1", porcelain.ResetSoft)
	assert.ErrorContains(t, err, "middle of a merge")
	_, err = porcelain.Reset(r, "HEAD", porcelain.ResetMixed)
	require.NoError(t, err, "a mixed reset clears the conflict")
}

func TestResetPathsUnstagesFiles(t *testing.T) {
	r := threeCommits(t)
	write(t, r, "a.txt", "edited\n")
	write(t, r, "new.txt", "n\n")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)

	changed, err := porcelain.ResetPaths(r, "", []string{"a.txt", "new.txt"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt", "new.txt"}, changed)

	s := status(t, r)
	assert.Empty(t, s.Staged)
	assert.Equal(t, []porcelain.Change{{Path: "a.txt", Kind: porcelain.Modified}}, s.Unstaged)
	assert.Equal(t, []string{"new.txt"}, s.Untracked)
	assert.Equal(t, "edited\n", read(t, r, "a.txt"), "the working tree is untouched")
}

func TestResetPathsFromAnotherRevisionAndDirectories(t *testing.T) {
	r := newRepo(t)
	write(t, r, "d/x", "1\n")
	commitAll(t, r, "one")
	write(t, r, "d/x", "2\n")
	commitAll(t, r, "two")

	changed, err := porcelain.ResetPaths(r, "HEAD~1", []string{"d"})
	require.NoError(t, err)
	assert.Equal(t, []string{"d/x"}, changed)
	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "d/x", Kind: porcelain.Modified}}, s.Staged)
	assert.Equal(t, []porcelain.Change{{Path: "d/x", Kind: porcelain.Modified}}, s.Unstaged)
}

func TestResetPathsBeforeFirstCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	_, err := porcelain.Add(r, []string{"a"})
	require.NoError(t, err)
	_, err = porcelain.ResetPaths(r, "", []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, status(t, r).Untracked)
}

func TestResetPathsClearsConflictsAndRejectsUnknownPaths(t *testing.T) {
	r := threeCommits(t)
	require.NoError(t, saveConflict(r, "a.txt"))
	_, err := porcelain.ResetPaths(r, "", []string{"a.txt"})
	require.NoError(t, err)
	assert.Empty(t, status(t, r).Unmerged)

	_, err = porcelain.ResetPaths(r, "", []string{"ghost"})
	assert.Error(t, err)
}

func TestResetPathsNothingToDo(t *testing.T) {
	r := threeCommits(t)
	changed, err := porcelain.ResetPaths(r, "", []string{"a.txt"})
	require.NoError(t, err)
	assert.Empty(t, changed)
}

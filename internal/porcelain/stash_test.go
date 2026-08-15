package porcelain_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

// dirtyRepo has a committed a.txt and b.txt, then an unstaged edit to
// a.txt, a staged edit to b.txt and a new untracked file.
func dirtyRepo(t *testing.T) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "a.txt", "a1\n")
	write(t, r, "b.txt", "b1\n")
	commitAll(t, r, "base")
	write(t, r, "a.txt", "a2 unstaged\n")
	write(t, r, "b.txt", "b2 staged\n")
	_, err := porcelain.Add(r, []string{"b.txt"})
	require.NoError(t, err)
	write(t, r, "untracked.txt", "u\n")
	return r
}

func TestStashPushSavesAndCleans(t *testing.T) {
	r := dirtyRepo(t)
	tick(t)
	entry, err := porcelain.StashPush(r, "")
	require.NoError(t, err)
	assert.Regexp(t, `^WIP on main: [0-9a-f]{7} base$`, entry.Message)

	assert.Equal(t, "a1\n", read(t, r, "a.txt"))
	assert.Equal(t, "b1\n", read(t, r, "b.txt"))
	s := status(t, r)
	assert.Empty(t, s.Staged)
	assert.Empty(t, s.Unstaged)
	assert.Equal(t, []string{"untracked.txt"}, s.Untracked, "untracked files are not stashed")

	// The stash commit has two parents: HEAD and the index commit.
	c := commitOf(t, r, entry.ID)
	require.Len(t, c.Parents, 2)
	assert.Equal(t, headID(t, r), c.Parents[0])
	indexCommit := commitOf(t, r, c.Parents[1])
	assert.Regexp(t, `^index on main: [0-9a-f]{7} base\n$`, indexCommit.Message)
	files, err := porcelain.StashShow(r, 0)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestStashPushWithMessageAndNothingToSave(t *testing.T) {
	r := dirtyRepo(t)
	tick(t)
	entry, err := porcelain.StashPush(r, "half done")
	require.NoError(t, err)
	assert.Equal(t, "On main: half done", entry.Message)

	_, err = porcelain.StashPush(r, "")
	assert.True(t, errors.Is(err, porcelain.ErrNoLocalChanges))
	_, err = porcelain.StashPush(newRepo(t), "")
	assert.ErrorContains(t, err, "initial commit")
}

func TestStashListIsNewestFirst(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "first")
	require.NoError(t, err)
	write(t, r, "a.txt", "again\n")
	_, err = porcelain.StashPush(r, "second")
	require.NoError(t, err)

	list, err := porcelain.StashList(r)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "On main: second", list[0].Message)
	assert.Equal(t, "On main: first", list[1].Message)
	assert.Equal(t, 1, list[1].Index)
}

func TestStashApplyBringsChangesBackUnstaged(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "")
	require.NoError(t, err)

	conflicts, err := porcelain.StashApply(r, 0)
	require.NoError(t, err)
	assert.Empty(t, conflicts)
	assert.Equal(t, "a2 unstaged\n", read(t, r, "a.txt"))
	assert.Equal(t, "b2 staged\n", read(t, r, "b.txt"))
	s := status(t, r)
	assert.Empty(t, s.Staged, "git apply does not restore the index either")
	assert.Equal(t, []porcelain.Change{{Path: "a.txt", Kind: porcelain.Modified}, {Path: "b.txt", Kind: porcelain.Modified}}, s.Unstaged)

	list, _ := porcelain.StashList(r)
	assert.Len(t, list, 1, "apply keeps the entry")
}

func TestStashApplyKeepsNewFilesStaged(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "base")
	write(t, r, "new.txt", "n\n")
	_, err := porcelain.Add(r, []string{"new.txt"})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("a"))
	_, err = porcelain.StashPush(r, "")
	require.NoError(t, err)
	assert.Equal(t, "1\n", read(t, r, "a"), "the deleted file is back")

	_, err = porcelain.StashApply(r, 0)
	require.NoError(t, err)
	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "new.txt", Kind: porcelain.Added}}, s.Staged)
	assert.Equal(t, []porcelain.Change{{Path: "a", Kind: porcelain.Deleted}}, s.Unstaged)
}

func TestStashPopDropsTheEntry(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "")
	require.NoError(t, err)
	conflicts, dropped, err := porcelain.StashPop(r, 0)
	require.NoError(t, err)
	assert.Empty(t, conflicts)
	assert.NotEmpty(t, dropped.Message)
	list, _ := porcelain.StashList(r)
	assert.Empty(t, list)
	assert.False(t, r.Refs.Exists("refs/stash"))
	assert.Equal(t, "a2 unstaged\n", read(t, r, "a.txt"))
}

func TestStashPopOntoAMovedBranchMergesChanges(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "")
	require.NoError(t, err)
	write(t, r, "c.txt", "c\n")
	commitAll(t, r, "other commit")

	_, _, err = porcelain.StashPop(r, 0)
	require.NoError(t, err)
	assert.Equal(t, "a2 unstaged\n", read(t, r, "a.txt"))
	assert.Equal(t, "c\n", read(t, r, "c.txt"))
}

func TestStashConflictKeepsTheEntry(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "")
	require.NoError(t, err)
	write(t, r, "a.txt", "a3 committed meanwhile\n")
	commitAll(t, r, "conflicting commit")

	conflicts, _, err := porcelain.StashPop(r, 0)
	require.NoError(t, err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, merge.ContentConflict, conflicts[0].Kind)
	assert.Contains(t, read(t, r, "a.txt"), "<<<<<<< Updated upstream")
	assert.Contains(t, read(t, r, "a.txt"), ">>>>>>> Stashed changes")
	list, _ := porcelain.StashList(r)
	assert.Len(t, list, 1, "a failed pop does not drop the stash")
}

func TestStashDropAndBadIndexes(t *testing.T) {
	r := dirtyRepo(t)
	_, err := porcelain.StashPush(r, "first")
	require.NoError(t, err)
	write(t, r, "a.txt", "more\n")
	_, err = porcelain.StashPush(r, "second")
	require.NoError(t, err)

	dropped, err := porcelain.StashDrop(r, 0)
	require.NoError(t, err)
	assert.Equal(t, "On main: second", dropped.Message)
	list, _ := porcelain.StashList(r)
	require.Len(t, list, 1)
	top, _ := r.Refs.Read("refs/stash")
	assert.Equal(t, list[0].ID, top, "refs/stash follows the new top")

	_, err = porcelain.StashDrop(r, 5)
	assert.True(t, errors.Is(err, porcelain.ErrNoStash))
	_, err = porcelain.StashApply(r, -1)
	assert.True(t, errors.Is(err, porcelain.ErrNoStash))
	_, err = porcelain.StashShow(r, 3)
	assert.True(t, errors.Is(err, porcelain.ErrNoStash))
}

// Real git reads our stash: list, show and pop agree with ours.
func TestRealGitReadsOurStash(t *testing.T) {
	testutil.RequireGit(t)
	r := dirtyRepo(t)
	tick(t)
	_, err := porcelain.StashPush(r, "")
	require.NoError(t, err)

	copyDir := t.TempDir()
	testutil.CopyDir(t, r.Root, copyDir)
	gitDir := filepath.Join(copyDir, ".hood")
	realGit := func(args ...string) string {
		return testutil.Git(t, copyDir, "", append([]string{"--git-dir=" + gitDir, "--work-tree=" + copyDir}, args...)...)
	}
	realGit("fsck", "--strict")
	list, err := porcelain.StashList(r)
	require.NoError(t, err)
	assert.Equal(t, "stash@{0}: "+list[0].Message+"\n", realGit("stash", "list"))
	assert.Equal(t, " a.txt | 2 +-\n b.txt | 2 +-\n 2 files changed, 2 insertions(+), 2 deletions(-)\n", realGit("stash", "show"))

	// Pop with git and with us must leave the same working tree.
	realGit("stash", "pop")
	_, _, err = porcelain.StashPop(r, 0)
	require.NoError(t, err)
	ours := strings.TrimSpace(testutil.Git(t, r.Root, "", "--git-dir="+r.GitDir, "--work-tree="+r.Root, "status", "--porcelain", "--untracked-files=no"))
	theirs := strings.TrimSpace(realGit("status", "--porcelain", "--untracked-files=no"))
	assert.Equal(t, theirs, ours)
	assert.Equal(t, read(t, r, "b.txt"), "b2 staged\n")
}

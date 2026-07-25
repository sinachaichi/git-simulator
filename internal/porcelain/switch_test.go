package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func read(t *testing.T, r *repo.Repo, path string) string {
	t.Helper()
	data, err := r.Work.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// feature adds b.txt and edits a.txt; main keeps the first version.
func branched(t *testing.T) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "a.txt", "one\n")
	commitAll(t, r, "first")
	_, err := porcelain.Switch(r, "feature", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	write(t, r, "a.txt", "two\n")
	write(t, r, "dir/b.txt", "b\n")
	commitAll(t, r, "feature work")
	return r
}

func TestSwitchUpdatesFilesIndexAndHead(t *testing.T) {
	r := branched(t)
	res, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Equal(t, "feature", res.FromBranch)
	assert.Equal(t, "main", res.ToBranch)
	assert.Equal(t, 2, res.Files)

	assert.Equal(t, "one\n", read(t, r, "a.txt"))
	assert.False(t, r.Work.Exists("dir/b.txt"))
	_, err = r.Work.Stat("dir")
	assert.Error(t, err, "the empty directory is removed")
	head, _ := r.Refs.Head()
	assert.Equal(t, "main", head.Branch())
	assert.True(t, status(t, r).Clean())

	_, err = porcelain.Switch(r, "feature", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Equal(t, "two\n", read(t, r, "a.txt"))
	assert.Equal(t, "b\n", read(t, r, "dir/b.txt"))
	assert.True(t, status(t, r).Clean())
}

func TestSwitchToCurrentBranchChangesNothing(t *testing.T) {
	r := branched(t)
	res, err := porcelain.Switch(r, "feature", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.True(t, res.AlreadyOn)
}

func TestSwitchWritesReflog(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	log, err := r.Refs.ReadLog("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "checkout: moving from feature to main", log[0].Message)
}

func TestSwitchCreateFromStartPoint(t *testing.T) {
	r := branched(t)
	res, err := porcelain.Switch(r, "hotfix", porcelain.SwitchOptions{Create: true, StartPoint: "main"})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, "one\n", read(t, r, "a.txt"))
	head, _ := r.Refs.Head()
	assert.Equal(t, "hotfix", head.Branch())

	_, err = porcelain.Switch(r, "hotfix", porcelain.SwitchOptions{Create: true})
	assert.True(t, errors.Is(err, porcelain.ErrBranchExists))
}

func TestSwitchCreateOnUnbornRepoJustRenamesHead(t *testing.T) {
	r := newRepo(t)
	_, err := porcelain.Switch(r, "trunk", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	head, _ := r.Refs.Head()
	assert.Equal(t, "trunk", head.Branch())
}

func TestSwitchDetach(t *testing.T) {
	r := branched(t)
	res, err := porcelain.Switch(r, "HEAD~1", porcelain.SwitchOptions{Detach: true})
	require.NoError(t, err)
	assert.True(t, res.Detached)
	head, _ := r.Refs.Head()
	assert.True(t, head.Detached())
	assert.Equal(t, "one\n", read(t, r, "a.txt"))
}

func TestSwitchStrictRejectsNonBranchesButCheckoutDetaches(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "HEAD~1", porcelain.SwitchOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrBranchNotFound))

	res, err := porcelain.Switch(r, "HEAD~1", porcelain.SwitchOptions{Lenient: true})
	require.NoError(t, err)
	assert.True(t, res.Detached)
}

func TestSwitchKeepsUnrelatedLocalChanges(t *testing.T) {
	r := branched(t)
	write(t, r, "other.txt", "mine\n") // untracked and not touched by the switch
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Equal(t, "mine\n", read(t, r, "other.txt"))
}

func TestSwitchRefusesEditsToFilesTheTargetDeletes(t *testing.T) {
	r := branched(t)
	write(t, r, "dir/b.txt", "b edited\n")
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	var lc *porcelain.LocalChangesError
	require.True(t, errors.As(err, &lc), "b.txt is deleted by the switch, so the edit would be lost")
	assert.Equal(t, []string{"dir/b.txt"}, lc.Paths)
}

func TestSwitchRefusesToLoseLocalModifications(t *testing.T) {
	r := branched(t)
	write(t, r, "a.txt", "dirty\n")
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	var lc *porcelain.LocalChangesError
	require.True(t, errors.As(err, &lc))
	assert.Equal(t, []string{"a.txt"}, lc.Paths)
	assert.False(t, lc.Untracked)

	head, _ := r.Refs.Head()
	assert.Equal(t, "feature", head.Branch(), "nothing moved")
	assert.Equal(t, "dirty\n", read(t, r, "a.txt"))
}

func TestSwitchRefusesStagedChanges(t *testing.T) {
	r := branched(t)
	write(t, r, "a.txt", "staged\n")
	_, err := porcelain.Add(r, []string{"a.txt"})
	require.NoError(t, err)
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	assert.ErrorContains(t, err, "a.txt")
}

func TestSwitchRefusesToOverwriteUntrackedFile(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	write(t, r, "dir/b.txt", "untracked and in the way\n")
	_, err = porcelain.Switch(r, "feature", porcelain.SwitchOptions{})
	var lc *porcelain.LocalChangesError
	require.True(t, errors.As(err, &lc))
	assert.True(t, lc.Untracked)
	assert.Equal(t, []string{"dir/b.txt"}, lc.Paths)
}

func TestSwitchAllowsStagedChangeThatMatchesTarget(t *testing.T) {
	r := branched(t)
	write(t, r, "a.txt", "one\n")
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	// a.txt on disk equals main's version, so nothing is lost.
	require.NoError(t, err)
}

func TestSwitchReplacesDirectoryWithFile(t *testing.T) {
	r := newRepo(t)
	write(t, r, "x/inner", "i\n")
	commitAll(t, r, "dir")
	_, err := porcelain.Switch(r, "file", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("x/inner"))
	write(t, r, "x", "now a file\n")
	commitAll(t, r, "file")

	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Equal(t, "i\n", read(t, r, "x/inner"))
	_, err = porcelain.Switch(r, "file", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Equal(t, "now a file\n", read(t, r, "x"))
}

func TestSwitchRestoresExecutableBit(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.Work.WriteFile("run.sh", []byte("#!/bin/sh\n"), 0o755))
	commitAll(t, r, "script")
	_, err := porcelain.Switch(r, "other", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("run.sh"))
	commitAll(t, r, "remove")
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	info, err := r.Work.Stat("run.sh")
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111)
}

func TestSwitchRefusesWithUnmergedIndex(t *testing.T) {
	r := branched(t)
	require.NoError(t, saveConflict(r, "a.txt"))
	_, err := porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrUnmergedIndex))
}

func TestSwitchUnknownTarget(t *testing.T) {
	r := branched(t)
	_, err := porcelain.Switch(r, "ghost", porcelain.SwitchOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrBranchNotFound))
	_, err = porcelain.Switch(r, "ghost", porcelain.SwitchOptions{Lenient: true})
	assert.Error(t, err)
}

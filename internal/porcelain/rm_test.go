package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestRmRemovesFromIndexAndDisk(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	write(t, r, "keep", "k\n")
	commitAll(t, r, "first")

	removed, err := porcelain.Rm(r, []string{"a"}, porcelain.RmOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, removed)
	assert.False(t, r.Work.Exists("a"))

	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "a", Kind: porcelain.Deleted}}, s.Staged)
	assert.Empty(t, s.Unstaged)
}

func TestRmCachedKeepsTheFile(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "first")
	_, err := porcelain.Rm(r, []string{"a"}, porcelain.RmOptions{Cached: true})
	require.NoError(t, err)
	assert.True(t, r.Work.Exists("a"))
	s := status(t, r)
	assert.Equal(t, []string{"a"}, s.Untracked)
	assert.Equal(t, []porcelain.Change{{Path: "a", Kind: porcelain.Deleted}}, s.Staged)
}

func TestRmRefusesLocalModificationsUnlessForced(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "first")
	write(t, r, "a", "edited\n")

	_, err := porcelain.Rm(r, []string{"a"}, porcelain.RmOptions{})
	var rmErr *porcelain.RmError
	require.True(t, errors.As(err, &rmErr))
	assert.Equal(t, []string{"a"}, rmErr.Paths)
	assert.Contains(t, rmErr.Reason, "local modifications")
	assert.True(t, r.Work.Exists("a"))

	_, err = porcelain.Rm(r, []string{"a"}, porcelain.RmOptions{Force: true})
	require.NoError(t, err)
	assert.False(t, r.Work.Exists("a"))
}

func TestRmRefusesStagedChanges(t *testing.T) {
	r := newRepo(t)
	write(t, r, "new", "n\n")
	_, err := porcelain.Add(r, []string{"new"})
	require.NoError(t, err)

	_, err = porcelain.Rm(r, []string{"new"}, porcelain.RmOptions{})
	var rmErr *porcelain.RmError
	require.True(t, errors.As(err, &rmErr))
	assert.Contains(t, rmErr.Reason, "staged changes")

	_, err = porcelain.Rm(r, []string{"new"}, porcelain.RmOptions{Cached: true})
	require.NoError(t, err, "--cached may drop a staged file that matches the disk")
	assert.True(t, r.Work.Exists("new"))
}

func TestRmCachedRefusesWhenIndexDiffersFromBoth(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "first")
	write(t, r, "a", "2\n")
	_, _ = porcelain.Add(r, []string{"a"})
	write(t, r, "a", "3\n")
	_, err := porcelain.Rm(r, []string{"a"}, porcelain.RmOptions{Cached: true})
	assert.Error(t, err)
}

func TestRmDirectoryNeedsRecursive(t *testing.T) {
	r := newRepo(t)
	write(t, r, "d/x", "1\n")
	write(t, r, "d/y", "2\n")
	write(t, r, "other", "o\n")
	commitAll(t, r, "first")

	_, err := porcelain.Rm(r, []string{"d"}, porcelain.RmOptions{})
	var rmErr *porcelain.RmError
	require.True(t, errors.As(err, &rmErr))
	assert.Contains(t, rmErr.Reason, "without -r")

	removed, err := porcelain.Rm(r, []string{"d"}, porcelain.RmOptions{Recursive: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"d/x", "d/y"}, removed)
	_, statErr := r.Work.Stat("d")
	assert.Error(t, statErr)
}

func TestRmUnknownPathAndNoArguments(t *testing.T) {
	r := newRepo(t)
	write(t, r, "untracked", "u\n")
	_, err := porcelain.Rm(r, []string{"untracked"}, porcelain.RmOptions{})
	var ps *porcelain.PathspecError
	assert.True(t, errors.As(err, &ps), "untracked files cannot be removed with rm")
	_, err = porcelain.Rm(r, nil, porcelain.RmOptions{})
	assert.Error(t, err)
}

func TestRmChangesNothingWhenOneSpecFails(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "first")
	_, err := porcelain.Rm(r, []string{"a", "ghost"}, porcelain.RmOptions{})
	assert.Error(t, err)
	assert.True(t, r.Work.Exists("a"))
	assert.True(t, status(t, r).Clean())
}

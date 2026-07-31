package porcelain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestShowCommitHasPatchAgainstParent(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "one\n")
	commitAll(t, r, "first")
	write(t, r, "a", "two\n")
	write(t, r, "b", "new\n")
	commitAll(t, r, "second")

	res, err := porcelain.Show(r, "HEAD")
	require.NoError(t, err)
	require.NotNil(t, res.Commit)
	assert.Equal(t, "second", res.Commit.Subject())
	assert.Equal(t, []string{"a", "b"}, paths(res.Files))
}

func TestShowRootCommitDiffsAgainstNothing(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "one\n")
	commitAll(t, r, "first")
	res, err := porcelain.Show(r, "HEAD")
	require.NoError(t, err)
	require.Len(t, res.Files, 1)
	assert.Zero(t, res.Files[0].OldMode)
}

func TestShowMergeCommitHasNoPatch(t *testing.T) {
	r := branched(t)
	merge := mergeCommit(t, r, revparseCommit(t, r, "main"), revparseCommit(t, r, "feature"))
	require.NoError(t, r.Refs.Update("refs/heads/feature", merge))

	res, err := porcelain.Show(r, "feature")
	require.NoError(t, err)
	assert.Len(t, res.Commit.Parents, 2)
	assert.Empty(t, res.Files)
}

func TestShowAnnotatedTagLeadsToItsCommit(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateTag(r, "v1", "", "release")
	require.NoError(t, err)
	res, err := porcelain.Show(r, "v1")
	require.NoError(t, err)
	require.NotNil(t, res.Tag)
	require.NotNil(t, res.Next)
	assert.Equal(t, "second", res.Next.Commit.Subject())
}

func TestShowTreeAndBlob(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "hello\n")
	commitAll(t, r, "first")
	res, err := porcelain.Show(r, "HEAD^{tree}")
	require.NoError(t, err)
	require.NotNil(t, res.Tree)
	assert.Equal(t, "a", res.Tree.Entries[0].Name)

	res, err = porcelain.Show(r, "ce01362")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(res.Blob.Data))
}

func TestShowUnknownRevision(t *testing.T) {
	_, err := porcelain.Show(twoCommits(t), "nope")
	assert.Error(t, err)
}

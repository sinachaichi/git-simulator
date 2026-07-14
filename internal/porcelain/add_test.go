package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestAddStagesFileAndStoresBlob(t *testing.T) {
	r := newRepo(t)
	write(t, r, "hello.txt", "hello\n")
	res, err := porcelain.Add(r, []string{"hello.txt"})
	require.NoError(t, err)
	assert.Equal(t, []string{"hello.txt"}, res.Added)

	ix, err := r.LoadIndex()
	require.NoError(t, err)
	e, ok := ix.Get("hello.txt")
	require.True(t, ok)
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a", e.ID.String())
	assert.Equal(t, objects.ModeFile, e.Mode)
	assert.True(t, r.ODB.Has(e.ID))
}

func TestAddKeepsContentFromTheMomentOfAdd(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.txt", "one\n")
	_, err := porcelain.Add(r, []string{"a.txt"})
	require.NoError(t, err)
	write(t, r, "a.txt", "two\n")

	ix, _ := r.LoadIndex()
	e, _ := ix.Get("a.txt")
	blob, err := r.ODB.ReadBlob(e.ID)
	require.NoError(t, err)
	assert.Equal(t, "one\n", string(blob.Data))
}

func TestAddDirectoryAndDot(t *testing.T) {
	r := newRepo(t)
	write(t, r, "src/a.go", "a")
	write(t, r, "src/deep/b.go", "b")
	write(t, r, "top.md", "t")

	res, err := porcelain.Add(r, []string{"src"})
	require.NoError(t, err)
	assert.Equal(t, []string{"src/a.go", "src/deep/b.go"}, res.Added)

	res, err = porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	assert.Equal(t, []string{"top.md"}, res.Added)
}

func TestAddReportsModifiedAndSkipsUnchanged(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	write(t, r, "b", "1")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	write(t, r, "a", "2")

	res, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, res.Modified)
	assert.Empty(t, res.Added)
}

func TestAddStagesDeletion(t *testing.T) {
	r := newRepo(t)
	write(t, r, "gone.txt", "x")
	_, err := porcelain.Add(r, []string{"gone.txt"})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("gone.txt"))

	res, err := porcelain.Add(r, []string{"gone.txt"})
	require.NoError(t, err)
	assert.Equal(t, []string{"gone.txt"}, res.Removed)
	ix, _ := r.LoadIndex()
	assert.Zero(t, ix.Len())
}

func TestAddUnknownPathFailsAndChangesNothing(t *testing.T) {
	r := newRepo(t)
	write(t, r, "real.txt", "x")
	_, err := porcelain.Add(r, []string{"real.txt", "nope.txt"})
	var ps *porcelain.PathspecError
	require.True(t, errors.As(err, &ps))
	assert.Equal(t, "nope.txt", ps.Spec)
	ix, _ := r.LoadIndex()
	assert.Zero(t, ix.Len(), "a failed add must not stage the other paths")
}

func TestAddNeedsAPath(t *testing.T) {
	_, err := porcelain.Add(newRepo(t), nil)
	assert.Error(t, err)
}

func TestAddTracksExecutableBit(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.Work.WriteFile("run.sh", []byte("#!/bin/sh\n"), 0o755))
	_, err := porcelain.Add(r, []string{"run.sh"})
	require.NoError(t, err)
	ix, _ := r.LoadIndex()
	e, _ := ix.Get("run.sh")
	assert.Equal(t, objects.ModeExecutable, e.Mode)
}

func TestAddNeverStagesMetaDir(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	ix, _ := r.LoadIndex()
	assert.Equal(t, []string{"a"}, ix.Paths())
}

func TestAddReportsDeletionWhenDirectoryBecomesFile(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a/b", "1")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	require.NoError(t, r.Work.Remove("a/b"))
	write(t, r, "a", "now a file")

	res, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	assert.Equal(t, []string{"a/b"}, res.Removed)
	assert.Equal(t, []string{"a"}, res.Added)
	ix, _ := r.LoadIndex()
	assert.Equal(t, []string{"a"}, ix.Paths())
}

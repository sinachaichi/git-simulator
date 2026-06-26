package repo_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/repo"
)

func TestInitCreatesLayout(t *testing.T) {
	dir := t.TempDir()
	r, err := repo.Init(dir, "main")
	require.NoError(t, err)
	for _, p := range []string{".hood/objects", ".hood/refs/heads", ".hood/refs/tags"} {
		assert.DirExists(t, filepath.Join(dir, p), p)
	}
	for _, p := range []string{".hood/HEAD", ".hood/config"} {
		assert.FileExists(t, filepath.Join(dir, p), p)
	}
	h, err := r.Refs.Head()
	require.NoError(t, err)
	assert.Equal(t, "main", h.Branch())
	assert.True(t, h.Unborn())
}

func TestInitCreatesMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "place")
	_, err := repo.Init(dir, "trunk")
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, ".hood"))
}

func TestInitTwiceFails(t *testing.T) {
	dir := t.TempDir()
	_, err := repo.Init(dir, "main")
	require.NoError(t, err)
	_, err = repo.Init(dir, "main")
	assert.True(t, errors.Is(err, repo.ErrAlreadyExists))
}

func TestInitRefusesRealGitRepos(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	for _, d := range []string{dir, sub} {
		_, err := repo.Init(d, "main")
		assert.True(t, errors.Is(err, repo.ErrInsideGit), d)
	}
	assert.NoDirExists(t, filepath.Join(sub, ".hood"))
}

func TestInitRejectsBadBranchName(t *testing.T) {
	_, err := repo.Init(t.TempDir(), "bad name")
	assert.Error(t, err)
}

func TestOpenAndDiscover(t *testing.T) {
	dir := t.TempDir()
	_, err := repo.Init(dir, "main")
	require.NoError(t, err)
	deep := filepath.Join(dir, "a", "b")
	require.NoError(t, os.MkdirAll(deep, 0o755))

	r, err := repo.Discover(deep)
	require.NoError(t, err)
	assert.Equal(t, dir, r.Root)

	_, err = repo.Open(deep)
	assert.True(t, errors.Is(err, repo.ErrNotARepo))
	_, err = repo.Discover(t.TempDir())
	assert.True(t, errors.Is(err, repo.ErrNotARepo))
}

func TestIndexLoadAndSave(t *testing.T) {
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	ix, err := r.LoadIndex()
	require.NoError(t, err)
	assert.Zero(t, ix.Len())
	require.NoError(t, r.SaveIndex(ix))
	assert.FileExists(t, r.IndexPath())
}

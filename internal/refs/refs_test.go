package refs_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
)

func id(s string) objects.ID { return objects.HashOf(&objects.Blob{Data: []byte(s)}) }

func newStore(t *testing.T) *refs.Store {
	t.Helper()
	return &refs.Store{Dir: t.TempDir()}
}

func TestUpdateAndRead(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Update("refs/heads/main", id("a")))
	got, err := s.Read("refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, id("a"), got)

	raw, err := os.ReadFile(filepath.Join(s.Dir, "refs", "heads", "main"))
	require.NoError(t, err)
	assert.Equal(t, id("a").String()+"\n", string(raw))
}

func TestUpdateOverwrites(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Update("refs/heads/main", id("a")))
	require.NoError(t, s.Update("refs/heads/main", id("b")))
	got, _ := s.Read("refs/heads/main")
	assert.Equal(t, id("b"), got)
	assert.NoFileExists(t, filepath.Join(s.Dir, "refs", "heads", "main.lock"))
}

func TestUpdateFailsWhenLocked(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Update("refs/heads/main", id("a")))
	lock := filepath.Join(s.Dir, "refs", "heads", "main.lock")
	require.NoError(t, os.WriteFile(lock, nil, 0o644))
	assert.ErrorContains(t, s.Update("refs/heads/main", id("b")), "cannot lock")
}

func TestReadMissing(t *testing.T) {
	_, err := newStore(t).Read("refs/heads/nope")
	assert.True(t, errors.Is(err, refs.ErrNotFound))
}

func TestReadCorrupt(t *testing.T) {
	s := newStore(t)
	require.NoError(t, os.MkdirAll(filepath.Join(s.Dir, "refs", "heads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(s.Dir, "refs", "heads", "x"), []byte("garbage\n"), 0o644))
	_, err := s.Read("refs/heads/x")
	assert.ErrorContains(t, err, "corrupt")
}

func TestDeletePrunesEmptyDirs(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Update("refs/heads/feat/login", id("a")))
	require.NoError(t, s.Delete("refs/heads/feat/login"))
	assert.NoDirExists(t, filepath.Join(s.Dir, "refs", "heads", "feat"))
	assert.True(t, errors.Is(s.Delete("refs/heads/feat/login"), refs.ErrNotFound))
}

func TestListSortedAndScoped(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.Update("refs/heads/main", id("a")))
	require.NoError(t, s.Update("refs/heads/feat/x", id("b")))
	require.NoError(t, s.Update("refs/tags/v1", id("c")))

	got, err := s.List(refs.BranchPrefix)
	require.NoError(t, err)
	assert.Equal(t, []refs.Ref{
		{Name: "refs/heads/feat/x", ID: id("b")},
		{Name: "refs/heads/main", ID: id("a")},
	}, got)

	got, err = s.List("refs/remotes/")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestValidName(t *testing.T) {
	good := []string{"refs/heads/main", "refs/heads/feat/three-way-merge", "refs/tags/v1.0.0", "refs/heads/a@b"}
	for _, n := range good {
		assert.NoError(t, refs.ValidName(n), n)
	}
	bad := []string{
		"", "@", "/refs/heads/x", "refs/heads/x/", "refs//x", "refs/heads/x.", "refs/heads/a..b",
		"refs/heads/a b", "refs/heads/a~1", "refs/heads/a^", "refs/heads/a:b", "refs/heads/a?", "refs/heads/a*",
		"refs/heads/a[", "refs/heads/a\\b", "refs/heads/.hidden", "refs/heads/x.lock", "refs/heads/a@{1}", "refs/heads/a\x01",
	}
	for _, n := range bad {
		assert.Error(t, refs.ValidName(n), n)
	}
}

func TestUpdateRejectsBadName(t *testing.T) {
	assert.Error(t, newStore(t).Update("refs/heads/a b", id("a")))
}

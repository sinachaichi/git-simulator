package index_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func TestSetConflictStoresThreeStages(t *testing.T) {
	ix := index.New()
	ix.Add(entry("f", "clean"))
	base, ours, theirs := entry("x", "base"), entry("x", "ours"), entry("x", "theirs")
	ix.SetConflict("f", &base, &ours, &theirs)

	_, ok := ix.Get("f")
	assert.False(t, ok, "stage 0 must be gone")
	st := ix.Stages("f")
	require.NotNil(t, st[0])
	assert.Equal(t, blob("base"), st[0].ID)
	assert.Equal(t, uint8(2), st[1].Stage)
	assert.Equal(t, blob("theirs"), st[2].ID)
	assert.Equal(t, []string{"f"}, ix.Conflicts())
	assert.Equal(t, 3, ix.Len())
}

func TestSetConflictWithMissingSides(t *testing.T) {
	ix := index.New()
	ours, theirs := entry("f", "ours"), entry("f", "theirs")
	ix.SetConflict("f", nil, &ours, &theirs)
	st := ix.Stages("f")
	assert.Nil(t, st[0])
	assert.NotNil(t, st[1])
	assert.Equal(t, 2, ix.Len())
}

func TestAddResolvesConflict(t *testing.T) {
	ix := index.New()
	base, ours, theirs := entry("f", "b"), entry("f", "o"), entry("f", "t")
	ix.SetConflict("f", &base, &ours, &theirs)
	ix.Add(entry("f", "resolved"))
	assert.Empty(t, ix.Conflicts())
	assert.Equal(t, 1, ix.Len())
}

func newStore(t *testing.T) *odb.Store {
	t.Helper()
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	return s
}

func TestWriteTreeThenFromTree(t *testing.T) {
	s := newStore(t)
	ix := index.New()
	for _, p := range []string{"README.md", "src/main.go", "src/util/x.go"} {
		e := entry(p, p)
		_, _, err := s.Write(&objects.Blob{Data: []byte(p)})
		require.NoError(t, err)
		ix.Add(e)
	}
	root, err := ix.WriteTree(s)
	require.NoError(t, err)

	back, err := index.FromTree(s, root)
	require.NoError(t, err)
	assert.Equal(t, ix.Paths(), back.Paths())
	for _, p := range ix.Paths() {
		a, _ := ix.Get(p)
		b, _ := back.Get(p)
		assert.Equal(t, a.ID, b.ID)
		assert.Equal(t, a.Mode, b.Mode)
	}
}

func TestWriteTreeRefusesUnmergedPaths(t *testing.T) {
	ix := index.New()
	ours, theirs := entry("f", "o"), entry("f", "t")
	ix.SetConflict("f", nil, &ours, &theirs)
	_, err := ix.WriteTree(newStore(t))
	var unmerged *index.UnmergedError
	require.True(t, errors.As(err, &unmerged))
	assert.Equal(t, []string{"f"}, unmerged.Paths)
}

// git write-tree on an index we wrote must give the same tree id.
func TestWriteTreeMatchesGitWriteTree(t *testing.T) {
	testutil.RequireGit(t)
	dir := testutil.BareRepo(t)
	s := &odb.Store{Dir: filepath.Join(dir, "objects")}
	ix := index.New()
	for _, p := range []string{"b.txt", "a/c.txt", "a-b.txt", "a/d/e.txt"} {
		_, _, err := s.Write(&objects.Blob{Data: []byte(p)})
		require.NoError(t, err)
		ix.Add(entry(p, p))
	}
	require.NoError(t, ix.Save(filepath.Join(dir, "index")))
	want := strings.TrimSpace(testutil.Git(t, dir, "", "write-tree"))
	got, err := ix.WriteTree(s)
	require.NoError(t, err)
	assert.Equal(t, want, got.String())
}

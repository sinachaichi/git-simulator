package history_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
)

func commit(t *testing.T, s *odb.Store, msg string, unix int64, parents ...objects.ID) objects.ID {
	t.Helper()
	tree, _, err := s.Write(&objects.Tree{})
	require.NoError(t, err)
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(unix, 0).In(time.UTC)}
	id, _, err := s.Write(&objects.Commit{Tree: tree, Parents: parents, Author: who, Committer: who, Message: msg + "\n"})
	require.NoError(t, err)
	return id
}

func msgs(nodes []history.Node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.Commit.Subject())
	}
	return out
}

func TestWalkLinear(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	a := commit(t, s, "a", 100)
	b := commit(t, s, "b", 200, a)
	c := commit(t, s, "c", 300, b)
	nodes, err := history.Walk(s, []objects.ID{c})
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "b", "a"}, msgs(nodes))
}

func TestWalkMergeVisitsEachCommitOnce(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	root := commit(t, s, "root", 100)
	left := commit(t, s, "left", 200, root)
	right := commit(t, s, "right", 250, root)
	merge := commit(t, s, "merge", 300, left, right)
	nodes, err := history.Walk(s, []objects.ID{merge})
	require.NoError(t, err)
	assert.Equal(t, []string{"merge", "right", "left", "root"}, msgs(nodes))
}

func TestWalkSameDateKeepsDiscoveryOrder(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	root := commit(t, s, "root", 100)
	one := commit(t, s, "one", 100, root)
	nodes, err := history.Walk(s, []objects.ID{one, root})
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "root"}, msgs(nodes))
}

func TestWalkFailsOnMissingParent(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	missing := objects.HashOf(&objects.Blob{Data: []byte("x")})
	c := commit(t, s, "orphan", 100, missing)
	_, err = history.Walk(s, []objects.ID{c})
	assert.ErrorIs(t, err, odb.ErrNotFound)
}

func TestIsAncestor(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	root := commit(t, s, "root", 100)
	left := commit(t, s, "left", 200, root)
	right := commit(t, s, "right", 250, root)
	merge := commit(t, s, "merge", 300, left, right)

	for _, tc := range []struct {
		anc, desc objects.ID
		want      bool
	}{
		{root, merge, true},
		{left, merge, true},
		{merge, merge, true},
		{merge, left, false},
		{left, right, false},
	} {
		got, err := history.IsAncestor(s, tc.anc, tc.desc)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestMergeBases(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	root := commit(t, s, "root", 100)
	a1 := commit(t, s, "a1", 200, root)
	b1 := commit(t, s, "b1", 210, root)
	a2 := commit(t, s, "a2", 300, a1)
	b2 := commit(t, s, "b2", 310, b1)
	merged := commit(t, s, "merged", 400, a2, b2)
	later := commit(t, s, "later", 500, a2)

	base := func(x, y objects.ID) []objects.ID {
		got, err := history.MergeBases(s, x, y)
		require.NoError(t, err)
		return got
	}
	assert.Equal(t, []objects.ID{root}, base(a2, b2))
	assert.Equal(t, []objects.ID{root}, base(b2, a2), "order does not matter")
	assert.Equal(t, []objects.ID{a2}, base(a2, later), "an ancestor is its own best base")
	assert.Equal(t, []objects.ID{a2}, base(later, merged))
	assert.Equal(t, []objects.ID{a2}, base(a2, a2))
}

func TestMergeBasesOfUnrelatedHistoriesIsEmpty(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	x := commit(t, s, "x", 100)
	y := commit(t, s, "y", 200)
	got, err := history.MergeBases(s, x, y)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestMergeBasesCrissCross(t *testing.T) {
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	root := commit(t, s, "root", 100)
	a := commit(t, s, "a", 200, root)
	b := commit(t, s, "b", 210, root)
	// Each side merges the other, which leaves two best bases.
	am := commit(t, s, "am", 300, a, b)
	bm := commit(t, s, "bm", 310, b, a)
	got, err := history.MergeBases(s, am, bm)
	require.NoError(t, err)
	assert.Equal(t, []objects.ID{b, a}, got, "both are best bases, newest first")
}

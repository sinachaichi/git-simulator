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

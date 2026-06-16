package refs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnbornHead(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.SetHeadBranch("main"))
	h, err := s.Head()
	require.NoError(t, err)
	assert.False(t, h.Detached())
	assert.True(t, h.Unborn())
	assert.Equal(t, "main", h.Branch())

	raw, err := os.ReadFile(filepath.Join(s.Dir, "HEAD"))
	require.NoError(t, err)
	assert.Equal(t, "ref: refs/heads/main\n", string(raw))
}

func TestHeadFollowsBranch(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.SetHeadBranch("main"))
	require.NoError(t, s.MoveHead(id("a")))

	h, err := s.Head()
	require.NoError(t, err)
	assert.Equal(t, id("a"), h.ID)
	branch, err := s.Read("refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, id("a"), branch)
}

func TestDetachedHead(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.SetHeadBranch("main"))
	require.NoError(t, s.MoveHead(id("a")))
	require.NoError(t, s.SetHeadDetached(id("b")))

	h, err := s.Head()
	require.NoError(t, err)
	assert.True(t, h.Detached())
	assert.Equal(t, id("b"), h.ID)
	assert.Equal(t, "", h.Branch())

	// Moving a detached HEAD must not touch the branch.
	require.NoError(t, s.MoveHead(id("c")))
	branch, _ := s.Read("refs/heads/main")
	assert.Equal(t, id("a"), branch)
	h, _ = s.Head()
	assert.Equal(t, id("c"), h.ID)
}

func TestHeadErrors(t *testing.T) {
	s := newStore(t)
	_, err := s.Head()
	assert.ErrorContains(t, err, "cannot read HEAD")

	require.NoError(t, os.WriteFile(filepath.Join(s.Dir, "HEAD"), []byte("nonsense\n"), 0o644))
	_, err = s.Head()
	assert.ErrorContains(t, err, "corrupt")

	assert.Error(t, s.SetHeadBranch("bad name"))
}

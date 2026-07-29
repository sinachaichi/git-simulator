package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestLightweightTagPointsAtTheCommit(t *testing.T) {
	r := twoCommits(t)
	head, _ := r.Refs.Head()
	id, err := porcelain.CreateTag(r, "v1", "", "")
	require.NoError(t, err)
	assert.Equal(t, head.ID, id)
	got, err := r.Refs.Read("refs/tags/v1")
	require.NoError(t, err)
	assert.Equal(t, head.ID, got)
}

func TestAnnotatedTagHasItsOwnObject(t *testing.T) {
	r := twoCommits(t)
	head, _ := r.Refs.Head()
	id, err := porcelain.CreateTag(r, "v1.0", "HEAD~1", "first release")
	require.NoError(t, err)
	assert.NotEqual(t, head.ID, id)

	o, err := r.ODB.Read(id)
	require.NoError(t, err)
	tag, ok := o.(*objects.Tag)
	require.True(t, ok)
	assert.Equal(t, "v1.0", tag.Name)
	assert.Equal(t, objects.TypeCommit, tag.Kind)
	assert.Equal(t, "first release\n", tag.Message)
	assert.Equal(t, "Ada Lovelace", tag.Tagger.Name)
	first := revparseCommit(t, r, "HEAD~1")
	assert.Equal(t, first, tag.Object)
}

func TestTagErrors(t *testing.T) {
	r := twoCommits(t)
	_, err := porcelain.CreateTag(r, "v1", "", "")
	require.NoError(t, err)
	_, err = porcelain.CreateTag(r, "v1", "", "")
	assert.True(t, errors.Is(err, porcelain.ErrTagExists))
	_, err = porcelain.CreateTag(r, "bad name", "", "")
	assert.Error(t, err)
	_, err = porcelain.CreateTag(r, "v2", "nope", "")
	assert.Error(t, err)
}

func TestListAndDeleteTags(t *testing.T) {
	r := twoCommits(t)
	_, _ = porcelain.CreateTag(r, "b-light", "", "")
	_, _ = porcelain.CreateTag(r, "a-note", "", "release notes\n\nmore")

	list, err := porcelain.ListTags(r)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "a-note", list[0].Name)
	assert.True(t, list[0].Annotated)
	assert.Equal(t, "release notes", list[0].Subject)
	assert.False(t, list[1].Annotated)

	_, err = porcelain.DeleteTag(r, "a-note")
	require.NoError(t, err)
	_, err = porcelain.DeleteTag(r, "a-note")
	assert.True(t, errors.Is(err, porcelain.ErrTagNotFound))
	list, _ = porcelain.ListTags(r)
	assert.Len(t, list, 1)
}

func TestAnnotatedTagResolvesAsACommit(t *testing.T) {
	r := twoCommits(t)
	head, _ := r.Refs.Head()
	_, err := porcelain.CreateTag(r, "v1", "", "note")
	require.NoError(t, err)

	nodes, err := porcelain.Log(r, porcelain.LogOptions{Revs: []string{"v1"}})
	require.NoError(t, err)
	assert.Equal(t, head.ID, nodes[0].ID)
	d, err := porcelain.Decorations(r)
	require.NoError(t, err)
	assert.Contains(t, d[head.ID], "tag: v1")
}

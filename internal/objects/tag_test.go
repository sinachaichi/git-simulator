package objects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

func TestTagIDMatchesGit(t *testing.T) {
	target, err := objects.ParseID("98a6b761e26c676fdc791373b564930f41248155")
	require.NoError(t, err)
	tag := &objects.Tag{
		Object:  target,
		Kind:    objects.TypeCommit,
		Name:    "v1.0",
		Tagger:  sig(1700000500, 7200),
		Message: "first release\n",
	}
	assert.Equal(t, "3212ef32dca1faa0e8eb1a699e23b200fca98aae", objects.HashOf(tag).String())

	got, err := objects.ParseTag(tag.Body())
	require.NoError(t, err)
	assert.Equal(t, tag.Body(), got.Body())
}

func TestParseTagErrors(t *testing.T) {
	_, err := objects.ParseTag([]byte("tag v1\n\nmsg"))
	assert.Error(t, err)
}

func TestParseDispatchesOnType(t *testing.T) {
	for _, o := range []objects.Object{
		&objects.Blob{Data: []byte("x")},
		&objects.Tree{},
		&objects.Commit{Tree: blobID("t"), Author: sig(1, 0), Committer: sig(1, 0), Message: "m\n"},
	} {
		got, err := objects.Parse(objects.Serialize(o))
		require.NoError(t, err)
		assert.Equal(t, o.Type(), got.Type())
		assert.Equal(t, objects.HashOf(o), objects.HashOf(got))
	}
}

func TestParseRejectsBadHeaders(t *testing.T) {
	for _, bad := range []string{"", "blob 3", "blob x\x00abc", "blob 5\x00abc", "thing 1\x00a", "blob\x00a"} {
		_, err := objects.Parse([]byte(bad))
		assert.Error(t, err, bad)
	}
}

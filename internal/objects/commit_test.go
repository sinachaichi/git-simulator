package objects_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

func sig(unix int64, zone int) objects.Signature {
	return objects.Signature{
		Name:  "Ada Lovelace",
		Email: "ada@example.com",
		When:  time.Unix(unix, 0).In(time.FixedZone("", zone)),
	}
}

func TestSignatureString(t *testing.T) {
	assert.Equal(t, "Ada Lovelace <ada@example.com> 1700000000 +0200", sig(1700000000, 7200).String())
	assert.Equal(t, "Ada Lovelace <ada@example.com> 1700000000 -0530", sig(1700000000, -(5*3600+1800)).String())
}

func TestParseSignatureRoundTrip(t *testing.T) {
	for _, zone := range []int{0, 7200, -(5*3600 + 1800)} {
		want := sig(1700000000, zone)
		got, err := objects.ParseSignature(want.String())
		require.NoError(t, err)
		assert.Equal(t, want.String(), got.String())
	}
}

func TestParseSignatureErrors(t *testing.T) {
	for _, bad := range []string{"", "no email 1 +0000", "A <a@b> notanumber +0000", "A <a@b> 1 0000", "A <a@b> 1 +00"} {
		_, err := objects.ParseSignature(bad)
		assert.Error(t, err, bad)
	}
}

// Expected id from real git with fixed dates:
//
//	GIT_AUTHOR_DATE="1700000000 +0000" GIT_COMMITTER_DATE="1700000000 +0000"
//	git commit-tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904 -m "empty"
//
// with name "Ada Lovelace" and email "ada@example.com".
func TestCommitIDMatchesGit(t *testing.T) {
	empty := objects.HashOf(&objects.Tree{})
	c := &objects.Commit{Tree: empty, Author: sig(1700000000, 0), Committer: sig(1700000000, 0), Message: "empty\n"}
	assert.Equal(t, "98a6b761e26c676fdc791373b564930f41248155", objects.HashOf(c).String())
}

func TestCommitRoundTrip(t *testing.T) {
	parent := blobID("p")
	c := &objects.Commit{
		Tree:      blobID("t"),
		Parents:   []objects.ID{parent, blobID("q")},
		Author:    sig(1700000000, 7200),
		Committer: sig(1700000100, 7200),
		Message:   "merge things\n\nlonger body\n",
	}
	got, err := objects.ParseCommit(c.Body())
	require.NoError(t, err)
	assert.Equal(t, c.Tree, got.Tree)
	assert.Equal(t, c.Parents, got.Parents)
	assert.Equal(t, c.Message, got.Message)
	assert.Equal(t, "merge things", got.Subject())
	assert.Equal(t, c.Body(), got.Body())
}

func TestParseCommitErrors(t *testing.T) {
	_, err := objects.ParseCommit([]byte("tree nothex\n\nmsg"))
	assert.Error(t, err)
	_, err = objects.ParseCommit([]byte("hello"))
	assert.Error(t, err)
}

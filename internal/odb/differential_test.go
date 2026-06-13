package odb_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func TestBlobsMatchHashObject(t *testing.T) {
	testutil.RequireGit(t)
	dir := testutil.BareRepo(t)
	contents := []string{"", "a", "hello\n", "line1\nline2\n", "tab\tand\x00nul", strings.Repeat("x", 100000)}
	for _, c := range contents {
		want := strings.TrimSpace(testutil.Git(t, dir, c, "hash-object", "--stdin"))
		got := objects.HashOf(&objects.Blob{Data: []byte(c)})
		assert.Equal(t, want, got.String(), "len %d", len(c))
	}
}

// Objects we write must be readable by real git, and git must be happy
// with the whole database.
func TestRealGitReadsOurObjects(t *testing.T) {
	testutil.RequireGit(t)
	dir := testutil.BareRepo(t)
	store := &odb.Store{Dir: filepath.Join(dir, "objects")}

	blob, _, err := store.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)
	tree := &objects.Tree{Entries: []objects.TreeEntry{
		{Mode: objects.ModeExecutable, Name: "run.sh", ID: blob},
		{Mode: objects.ModeFile, Name: "hello.txt", ID: blob},
	}}
	tree.Sort()
	treeID, _, err := store.Write(tree)
	require.NoError(t, err)

	when := time.Unix(1700000000, 0).In(time.FixedZone("", 0))
	who := objects.Signature{Name: "Ada Lovelace", Email: "ada@example.com", When: when}
	commit := &objects.Commit{Tree: treeID, Author: who, Committer: who, Message: "first\n"}
	commitID, _, err := store.Write(commit)
	require.NoError(t, err)

	assert.Equal(t, "hello\n", testutil.Git(t, dir, "", "cat-file", "-p", blob.String()))
	assert.Equal(t, "commit", strings.TrimSpace(testutil.Git(t, dir, "", "cat-file", "-t", commitID.String())))
	listing := testutil.Git(t, dir, "", "ls-tree", treeID.String())
	assert.Contains(t, listing, "100644 blob "+blob.String()+"\thello.txt")
	assert.Contains(t, listing, "100755 blob "+blob.String()+"\trun.sh")
	testutil.Git(t, dir, "", "fsck", "--strict")
}

// The other direction: ids that real git computes for a tree and a commit
// must equal ours for the same input.
func TestTreeAndCommitIDsMatchGit(t *testing.T) {
	testutil.RequireGit(t)
	dir := testutil.BareRepo(t)

	blob := strings.TrimSpace(testutil.Git(t, dir, "hello\n", "hash-object", "-w", "--stdin"))
	blobID, err := objects.ParseID(blob)
	require.NoError(t, err)

	mktree := "100644 blob " + blob + "\tb.txt\n" +
		"100644 blob " + blob + "\ta.txt\n" +
		"100755 blob " + blob + "\trun.sh\n"
	wantTree := strings.TrimSpace(testutil.Git(t, dir, mktree, "mktree"))

	tree := &objects.Tree{Entries: []objects.TreeEntry{
		{Mode: objects.ModeFile, Name: "b.txt", ID: blobID},
		{Mode: objects.ModeFile, Name: "a.txt", ID: blobID},
		{Mode: objects.ModeExecutable, Name: "run.sh", ID: blobID},
	}}
	tree.Sort()
	assert.Equal(t, wantTree, objects.HashOf(tree).String())

	when := time.Unix(1700000000, 0).In(time.FixedZone("", 0))
	who := objects.Signature{Name: "Ada Lovelace", Email: "ada@example.com", When: when}
	c := &objects.Commit{Tree: objects.HashOf(tree), Author: who, Committer: who, Message: "multi\n\nline body\n"}
	wantCommit := strings.TrimSpace(testutil.Git(t, dir, "", "commit-tree", wantTree, "-m", "multi\n\nline body"))
	assert.Equal(t, wantCommit, objects.HashOf(c).String())
}

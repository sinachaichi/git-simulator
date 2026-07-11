package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestFirstCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "hello.txt", "hello\n")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)

	res, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "first"})
	require.NoError(t, err)
	assert.True(t, res.Root)
	assert.Equal(t, "main", res.Branch)
	assert.Equal(t, "first\n", res.Commit.Message)

	head, err := r.Refs.Head()
	require.NoError(t, err)
	assert.Equal(t, res.ID, head.ID)
	main, err := r.Refs.Read("refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, res.ID, main)

	// The tree holds exactly the staged blob.
	tree, err := r.ODB.ReadTree(res.Commit.Tree)
	require.NoError(t, err)
	require.Len(t, tree.Entries, 1)
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a", tree.Entries[0].ID.String())
}

func TestSecondCommitHasParent(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	first, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	require.NoError(t, err)

	tick(t)
	write(t, r, "a", "2")
	_, _ = porcelain.Add(r, []string{"."})
	second, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "two"})
	require.NoError(t, err)
	assert.False(t, second.Root)
	assert.Equal(t, []objects.ID{first.ID}, second.Commit.Parents)
}

func TestCommitUsesStagedContentNotWorkingTree(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "staged")
	_, _ = porcelain.Add(r, []string{"a"})
	write(t, r, "a", "edited later")
	res, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "m"})
	require.NoError(t, err)
	tree, _ := r.ODB.ReadTree(res.Commit.Tree)
	blob, err := r.ODB.ReadBlob(tree.Entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "staged", string(blob.Data))
}

func TestNothingToCommit(t *testing.T) {
	r := newRepo(t)
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "empty"})
	assert.True(t, errors.Is(err, porcelain.ErrNothingToCommit))

	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	require.NoError(t, err)
	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: "again"})
	assert.True(t, errors.Is(err, porcelain.ErrNothingToCommit))
}

func TestAllowEmpty(t *testing.T) {
	r := newRepo(t)
	res, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "empty root", AllowEmpty: true})
	require.NoError(t, err)
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", res.Commit.Tree.String())
}

func TestEmptyMessage(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: " \n\n"})
	assert.True(t, errors.Is(err, porcelain.ErrEmptyMessage))
}

func TestCommitAllStagesTrackedChangesOnly(t *testing.T) {
	r := newRepo(t)
	write(t, r, "tracked", "1")
	write(t, r, "gone", "1")
	_, _ = porcelain.Add(r, []string{"."})
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	require.NoError(t, err)

	tick(t)
	write(t, r, "tracked", "2")
	write(t, r, "untracked", "x")
	require.NoError(t, r.Work.Remove("gone"))
	res, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "two", All: true})
	require.NoError(t, err)

	tree, _ := r.ODB.ReadTree(res.Commit.Tree)
	require.Len(t, tree.Entries, 1)
	assert.Equal(t, "tracked", tree.Entries[0].Name)
}

func TestAmendKeepsParentAndAuthor(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	first, _ := porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	tick(t)
	write(t, r, "a", "2")
	_, _ = porcelain.Add(r, []string{"."})
	second, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "two"})
	require.NoError(t, err)

	tick(t)
	amended, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "two, fixed", Amend: true})
	require.NoError(t, err)
	assert.True(t, amended.Amended)
	assert.Equal(t, []objects.ID{first.ID}, amended.Commit.Parents)
	assert.Equal(t, second.Commit.Author, amended.Commit.Author)
	assert.NotEqual(t, second.Commit.Committer.When, amended.Commit.Committer.When)
	assert.NotEqual(t, second.ID, amended.ID)

	// Without a message the old one stays.
	again, err := porcelain.Commit(r, porcelain.CommitOptions{Amend: true})
	require.NoError(t, err)
	assert.Equal(t, "two, fixed\n", again.Commit.Message)
}

func TestAmendWithoutCommitFails(t *testing.T) {
	r := newRepo(t)
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "x", Amend: true})
	assert.ErrorContains(t, err, "no commit yet")
}

func TestCommitRefusesUnmergedPaths(t *testing.T) {
	r := newRepo(t)
	ix := index.New()
	ours := index.Entry{Path: "f", Mode: objects.ModeFile}
	ix.SetConflict("f", nil, &ours, &ours)
	require.NoError(t, r.SaveIndex(ix))
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "x"})
	var unmerged *index.UnmergedError
	assert.True(t, errors.As(err, &unmerged))
}

func TestCommitOnDetachedHeadMovesOnlyHead(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	first, _ := porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	require.NoError(t, r.Refs.SetHeadDetached(first.ID))

	tick(t)
	write(t, r, "a", "2")
	_, _ = porcelain.Add(r, []string{"."})
	res, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "two"})
	require.NoError(t, err)
	assert.Equal(t, "", res.Branch)
	main, _ := r.Refs.Read("refs/heads/main")
	assert.Equal(t, first.ID, main)
	head, _ := r.Refs.Head()
	assert.Equal(t, res.ID, head.ID)
}

func TestCommitWritesReflogs(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"."})
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: "first thing\n\nbody"})
	require.NoError(t, err)
	tick(t)
	write(t, r, "a", "2")
	_, _ = porcelain.Add(r, []string{"."})
	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: "second"})
	require.NoError(t, err)

	for _, name := range []string{"HEAD", "refs/heads/main"} {
		log, err := r.Refs.ReadLog(name)
		require.NoError(t, err)
		require.Len(t, log, 2, name)
		assert.Equal(t, "commit: second", log[0].Message)
		assert.Equal(t, "commit (initial): first thing", log[1].Message)
	}
}

func TestCleanMessage(t *testing.T) {
	for in, want := range map[string]string{
		"hello":                   "hello\n",
		"hello\n\n\n":             "hello\n",
		"\n\nhello  \n":           "hello\n",
		"a\n\n\n\nb":              "a\n\nb\n",
		"subject\r\n\r\nbody\r\n": "subject\n\nbody\n",
		"   \n":                   "",
		"":                        "",
	} {
		assert.Equal(t, want, porcelain.CleanMessage(in), "%q", in)
	}
}

package porcelain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func commitAll(t *testing.T, r *repo.Repo, msg string) {
	t.Helper()
	tick(t)
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: msg})
	require.NoError(t, err)
}

func status(t *testing.T, r *repo.Repo) porcelain.StatusResult {
	t.Helper()
	s, err := porcelain.Status(r)
	require.NoError(t, err)
	return s
}

func TestStatusOnEmptyRepo(t *testing.T) {
	r := newRepo(t)
	s := status(t, r)
	assert.True(t, s.Clean())
	assert.True(t, s.Unborn)
	assert.Equal(t, "main", s.Branch)
}

func TestStatusUntrackedBeforeFirstCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "b.txt", "b")
	write(t, r, "a/c.txt", "c")
	s := status(t, r)
	assert.Equal(t, []string{"a/c.txt", "b.txt"}, s.Untracked)
	assert.Empty(t, s.Staged)
}

func TestStatusStagedNewFile(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	_, _ = porcelain.Add(r, []string{"a"})
	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "a", Kind: porcelain.Added}}, s.Staged)
	assert.Empty(t, s.Unstaged)
	assert.Empty(t, s.Untracked)
}

func TestStatusCleanAfterCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	commitAll(t, r, "one")
	s := status(t, r)
	assert.True(t, s.Clean())
	assert.False(t, s.Unborn)
}

// These mirror the old TestStatusModified, Staged, Mixed1 and Mixed2.
func TestStatusModifiedNotStaged(t *testing.T) {
	r := newRepo(t)
	write(t, r, "README.md", "### MY GIT IMPL")
	commitAll(t, r, "initial commit")
	require.NoError(t, r.Work.AppendFile("README.md", []byte("\nhello world")))

	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "README.md", Kind: porcelain.Modified}}, s.Unstaged)
	assert.Empty(t, s.Staged)
}

func TestStatusModifiedAndStaged(t *testing.T) {
	r := newRepo(t)
	write(t, r, "README.md", "### MY GIT IMPL")
	commitAll(t, r, "initial commit")
	require.NoError(t, r.Work.AppendFile("README.md", []byte("\nhello world")))
	_, _ = porcelain.Add(r, []string{"README.md"})

	s := status(t, r)
	assert.Empty(t, s.Unstaged)
	assert.Equal(t, []porcelain.Change{{Path: "README.md", Kind: porcelain.Modified}}, s.Staged)
}

func TestStatusMixed(t *testing.T) {
	r := newRepo(t)
	write(t, r, "README.md", "### MY GIT IMPL")
	commitAll(t, r, "initial commit")
	write(t, r, ".gitignore", "*.class")
	require.NoError(t, r.Work.AppendFile("README.md", []byte("\nhello world")))
	_, _ = porcelain.Add(r, []string{"README.md"})
	require.NoError(t, r.Work.AppendFile("README.md", []byte("\nit's time to goodbye")))

	s := status(t, r)
	assert.Equal(t, []string{".gitignore"}, s.Untracked)
	assert.Equal(t, []porcelain.Change{{Path: "README.md", Kind: porcelain.Modified}}, s.Staged)
	assert.Equal(t, []porcelain.Change{{Path: "README.md", Kind: porcelain.Modified}}, s.Unstaged)
}

func TestStatusDeletedFiles(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	write(t, r, "b", "1")
	commitAll(t, r, "one")
	require.NoError(t, r.Work.Remove("a"))
	require.NoError(t, r.Work.Remove("b"))
	_, _ = porcelain.Add(r, []string{"b"})

	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "a", Kind: porcelain.Deleted}}, s.Unstaged)
	assert.Equal(t, []porcelain.Change{{Path: "b", Kind: porcelain.Deleted}}, s.Staged)
}

func TestStatusDetectsModeChange(t *testing.T) {
	r := newRepo(t)
	write(t, r, "run.sh", "#!/bin/sh")
	commitAll(t, r, "one")
	require.NoError(t, r.Work.WriteFile("run.sh", []byte("#!/bin/sh"), 0o755))
	s := status(t, r)
	assert.Equal(t, []porcelain.Change{{Path: "run.sh", Kind: porcelain.Modified}}, s.Unstaged)
}

func TestStatusSameContentDifferentTimeIsClean(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	commitAll(t, r, "one")
	write(t, r, "a", "1") // rewrite with identical content
	assert.True(t, status(t, r).Clean())
}

func TestStatusReportsDetachedHead(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	commitAll(t, r, "one")
	head, _ := r.Refs.Head()
	require.NoError(t, r.Refs.SetHeadDetached(head.ID))
	s := status(t, r)
	assert.True(t, s.Detached)
	assert.Equal(t, head.ID, s.HeadID)
}

func TestStatusUnmergedKinds(t *testing.T) {
	r := newRepo(t)
	e := func(c string) *index.Entry {
		return &index.Entry{Mode: objects.ModeFile, ID: objects.HashOf(&objects.Blob{Data: []byte(c)})}
	}
	ix := index.New()
	ix.SetConflict("both-mod", e("b"), e("o"), e("t"))
	ix.SetConflict("both-add", nil, e("o"), e("t"))
	ix.SetConflict("del-us", e("b"), nil, e("t"))
	ix.SetConflict("del-them", e("b"), e("o"), nil)
	ix.SetConflict("add-us", nil, e("o"), nil)
	ix.SetConflict("add-them", nil, nil, e("t"))
	require.NoError(t, r.SaveIndex(ix))

	got := map[string]porcelain.ConflictKind{}
	for _, u := range status(t, r).Unmerged {
		got[u.Path] = u.Kind
	}
	assert.Equal(t, map[string]porcelain.ConflictKind{
		"both-mod": porcelain.BothModified, "both-add": porcelain.BothAdded,
		"del-us": porcelain.DeletedByUs, "del-them": porcelain.DeletedByThem,
		"add-us": porcelain.AddedByUs, "add-them": porcelain.AddedByThem,
	}, got)
}

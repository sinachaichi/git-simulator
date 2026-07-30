package porcelain_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/revparse"

	"github.com/sinachaichi/git-simulator/internal/repo"
)

// clock gives every test commit a new, increasing time so history order
// is predictable.
var clock int64 = 1700000000

func tick(t *testing.T) {
	t.Helper()
	clock += 3600
	date := fmt.Sprintf("%d +0000", clock)
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
}

func newRepo(t *testing.T) *repo.Repo {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Ada Lovelace")
	t.Setenv("GIT_AUTHOR_EMAIL", "ada@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ada Lovelace")
	t.Setenv("GIT_COMMITTER_EMAIL", "ada@example.com")
	tick(t)
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	return r
}

func write(t *testing.T, r *repo.Repo, path, content string) {
	t.Helper()
	require.NoError(t, r.Work.WriteFile(path, []byte(content), 0o644))
}

// saveConflict puts conflict stages for path into the index.
func saveConflict(r *repo.Repo, path string) error {
	ix, err := r.LoadIndex()
	if err != nil {
		return err
	}
	e := index.Entry{Mode: objects.ModeFile, ID: objects.HashOf(&objects.Blob{Data: []byte(path)})}
	ix.SetConflict(path, &e, &e, &e)
	return r.SaveIndex(ix)
}

func revparseCommit(t *testing.T, r *repo.Repo, rev string) objects.ID {
	t.Helper()
	id, err := revparse.ResolveCommit(r, rev)
	require.NoError(t, err)
	return id
}

// mergeCommit writes a merge commit of two commits with the tree of the
// second one. It does not touch refs or the working tree.
func mergeCommit(t *testing.T, r *repo.Repo, first, second objects.ID) objects.ID {
	t.Helper()
	c, err := r.ODB.ReadCommit(second)
	require.NoError(t, err)
	merge := *c
	merge.Parents = []objects.ID{first, second}
	merge.Message = "merge\n"
	id, _, err := r.ODB.Write(&merge)
	require.NoError(t, err)
	return id
}

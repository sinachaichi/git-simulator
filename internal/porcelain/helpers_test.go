package porcelain_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

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

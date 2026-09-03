package repl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func TestCompleteCommandNames(t *testing.T) {
	s := &Shell{cwd: t.TempDir()}
	assert.Empty(t, s.complete("zzz"))
	got := s.complete("sta")
	assert.Equal(t, []string{"stash", "status"}, got)
	assert.Contains(t, s.complete("xr"), "xray")
	assert.Empty(t, s.complete("status"), "an exact match has nothing left to add")
}

func TestCompleteBranchesTagsAndFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_AUTHOR_NAME", "A")
	t.Setenv("GIT_AUTHOR_EMAIL", "a@b.c")
	t.Setenv("GIT_COMMITTER_NAME", "A")
	t.Setenv("GIT_COMMITTER_EMAIL", "a@b.c")
	r, err := repo.Init(dir, "main")
	require.NoError(t, err)
	require.NoError(t, r.Work.WriteFile("readme.md", []byte("x"), 0o644))
	require.NoError(t, r.Work.WriteFile("src/main.go", []byte("x"), 0o644))
	_, err = porcelain.Add(r, []string{"."})
	require.NoError(t, err)
	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: "one"})
	require.NoError(t, err)
	_, err = porcelain.CreateBranch(r, "feature/login", "")
	require.NoError(t, err)
	_, err = porcelain.CreateTag(r, "v1.0", "", "")
	require.NoError(t, err)

	s := &Shell{cwd: dir}
	assert.Equal(t, []string{"switch feature/login"}, s.complete("switch fe"))
	assert.Equal(t, []string{"log v1.0"}, s.complete("log v1"))
	assert.Equal(t, []string{"add readme.md"}, s.complete("add re"))
	assert.Equal(t, []string{"add src/main.go"}, s.complete("add sr"))
	assert.Equal(t, []string{"merge main"}, s.complete("merge ma"))
}

func TestCompleteOutsideARepositoryOffersNothingForArguments(t *testing.T) {
	s := &Shell{cwd: t.TempDir()}
	assert.Empty(t, s.complete("switch ma"))
}

package porcelain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func subjects(nodes []history.Node) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.Commit.Subject())
	}
	return out
}

func linearRepo(t *testing.T, msgs ...string) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	for i, m := range msgs {
		write(t, r, "f.txt", m+string(rune('a'+i)))
		commitAll(t, r, m)
	}
	return r
}

// Ported from the old TestLog1 and TestLog2.
func TestLogNewestFirst(t *testing.T) {
	r := linearRepo(t, "commit1", "commit2", "commit3", "commit4")
	nodes, err := porcelain.Log(r, porcelain.LogOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"commit4", "commit3", "commit2", "commit1"}, subjects(nodes))
}

func TestLogOnEmptyRepoFails(t *testing.T) {
	_, err := porcelain.Log(newRepo(t), porcelain.LogOptions{})
	assert.ErrorContains(t, err, "no commits")
}

func TestLogMax(t *testing.T) {
	r := linearRepo(t, "a", "b", "c")
	nodes, err := porcelain.Log(r, porcelain.LogOptions{Max: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"c", "b"}, subjects(nodes))
}

func TestLogFromRevision(t *testing.T) {
	r := linearRepo(t, "a", "b", "c")
	nodes, err := porcelain.Log(r, porcelain.LogOptions{Revs: []string{"HEAD~1"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, subjects(nodes))

	_, err = porcelain.Log(r, porcelain.LogOptions{Revs: []string{"nope"}})
	assert.Error(t, err)
}

func TestLogAllIncludesOtherBranches(t *testing.T) {
	r := linearRepo(t, "a", "b")
	head, _ := r.Refs.Head()
	// A side branch with one extra commit made later than "b".
	tick(t)
	side := &objects.Commit{}
	base, err := r.ODB.ReadCommit(head.ID)
	require.NoError(t, err)
	*side = *base
	side.Parents = []objects.ID{head.ID}
	side.Message = "side\n"
	side.Committer.When = base.Committer.When.Add(60e9)
	id, _, err := r.ODB.Write(side)
	require.NoError(t, err)
	require.NoError(t, r.Refs.Update("refs/heads/side", id))

	nodes, err := porcelain.Log(r, porcelain.LogOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, subjects(nodes))
	nodes, err = porcelain.Log(r, porcelain.LogOptions{All: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"side", "b", "a"}, subjects(nodes))
}

func TestDecorations(t *testing.T) {
	r := linearRepo(t, "a", "b")
	head, _ := r.Refs.Head()
	require.NoError(t, r.Refs.Update("refs/heads/feature", head.ID))
	require.NoError(t, r.Refs.Update("refs/tags/v1", head.ID))
	require.NoError(t, r.Refs.Update("refs/tags/v2", head.ID))
	require.NoError(t, r.Refs.Update("refs/heads/aaa", head.ID))

	d, err := porcelain.Decorations(r)
	require.NoError(t, err)
	// Same order as real git: tags and branches in reverse name order.
	assert.Equal(t, []string{"HEAD -> main", "tag: v2", "tag: v1", "feature", "aaa"}, d[head.ID])

	require.NoError(t, r.Refs.SetHeadDetached(head.ID))
	d, err = porcelain.Decorations(r)
	require.NoError(t, err)
	assert.Equal(t, []string{"HEAD", "tag: v2", "tag: v1", "main", "feature", "aaa"}, d[head.ID])
}

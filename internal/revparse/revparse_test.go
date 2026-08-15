package revparse_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

var clock = int64(1700000000)

func commit(t *testing.T, r *repo.Repo, msg string, parents ...objects.ID) objects.ID {
	t.Helper()
	tree, _, err := r.ODB.Write(&objects.Tree{})
	require.NoError(t, err)
	clock += 60
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(clock, 0).In(time.UTC)}
	id, _, err := r.ODB.Write(&objects.Commit{Tree: tree, Parents: parents, Author: who, Committer: who, Message: msg + "\n"})
	require.NoError(t, err)
	return id
}

// history builds:  c1 <- c2 <- c3 (main)   and   c2 <- f1 (feature)
// plus a merge m with parents c3 and f1 on branch "merged".
type fixture struct {
	r              *repo.Repo
	c1, c2, c3, f1 objects.ID
	m              objects.ID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	f := fixture{r: r}
	f.c1 = commit(t, r, "c1")
	f.c2 = commit(t, r, "c2", f.c1)
	f.c3 = commit(t, r, "c3", f.c2)
	f.f1 = commit(t, r, "f1", f.c2)
	f.m = commit(t, r, "merge", f.c3, f.f1)
	require.NoError(t, r.Refs.Update("refs/heads/main", f.c3))
	require.NoError(t, r.Refs.Update("refs/heads/feature", f.f1))
	require.NoError(t, r.Refs.Update("refs/heads/merged", f.m))
	return f
}

func TestResolveNames(t *testing.T) {
	f := newFixture(t)
	for expr, want := range map[string]objects.ID{
		"HEAD":                 f.c3,
		"@":                    f.c3,
		"main":                 f.c3,
		"feature":              f.f1,
		"heads/feature":        f.f1,
		"refs/heads/feature":   f.f1,
		f.c1.String():          f.c1,
		f.c1.String()[:7]:      f.c1,
		f.c1.String()[:4]:      f.c1,
		"HEAD~1":               f.c2,
		"HEAD~2":               f.c1,
		"HEAD~0":               f.c3,
		"HEAD^":                f.c2,
		"HEAD^^":               f.c1,
		"main~1^":              f.c1,
		"merged^":              f.c3,
		"merged^1":             f.c3,
		"merged^2":             f.f1,
		"merged^2^":            f.c2,
		"merged~1":             f.c3,
		"merged~2":             f.c2,
		"merged^0":             f.m,
		"merged^{commit}":      f.m,
		f.m.String()[:8] + "~": f.c3,
	} {
		got, err := revparse.Resolve(f.r, expr)
		require.NoError(t, err, expr)
		assert.Equal(t, want, got, expr)
	}
}

func TestResolveDetachedHead(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.r.Refs.SetHeadDetached(f.f1))
	got, err := revparse.Resolve(f.r, "HEAD^")
	require.NoError(t, err)
	assert.Equal(t, f.c2, got)
}

func TestResolveTree(t *testing.T) {
	f := newFixture(t)
	got, err := revparse.Resolve(f.r, "HEAD^{tree}")
	require.NoError(t, err)
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", got.String())
}

func TestResolveReflog(t *testing.T) {
	f := newFixture(t)
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(1, 0).In(time.UTC)}
	require.NoError(t, f.r.Refs.AppendLog("HEAD", refs.LogEntry{New: f.c1, Who: who, Message: "one"}))
	require.NoError(t, f.r.Refs.AppendLog("HEAD", refs.LogEntry{Old: f.c1, New: f.c2, Who: who, Message: "two"}))

	got, err := revparse.Resolve(f.r, "HEAD@{0}")
	require.NoError(t, err)
	assert.Equal(t, f.c2, got)
	got, err = revparse.Resolve(f.r, "@{1}")
	require.NoError(t, err)
	assert.Equal(t, f.c1, got)
	got, err = revparse.Resolve(f.r, "@{1}~0")
	require.NoError(t, err)
	assert.Equal(t, f.c1, got)

	_, err = revparse.Resolve(f.r, "HEAD@{5}")
	assert.ErrorContains(t, err, "only 2 reflog entries")
	_, err = revparse.Resolve(f.r, "HEAD@{yesterday}")
	assert.ErrorContains(t, err, "bad reflog index")
}

func TestResolveErrors(t *testing.T) {
	f := newFixture(t)
	for _, expr := range []string{"nope", "", "~1", "ffffffff", "zzzz"} {
		_, err := revparse.Resolve(f.r, expr)
		var nf *revparse.NotFoundError
		assert.True(t, errors.As(err, &nf), expr)
	}
	_, err := revparse.Resolve(f.r, "main^2")
	assert.ErrorContains(t, err, "has 1 parent(s), no parent number 2")
	_, err = revparse.Resolve(f.r, "c1~9")
	assert.Error(t, err)
	_, err = revparse.Resolve(f.r, "HEAD^{blob}")
	assert.ErrorContains(t, err, "unsupported peel type")
}

func TestResolveUnbornHead(t *testing.T) {
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	_, err = revparse.Resolve(r, "HEAD")
	assert.ErrorContains(t, err, "no commits")
}

func TestResolveAnnotatedTagPeelsForCommitOperators(t *testing.T) {
	f := newFixture(t)
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(5, 0).In(time.UTC)}
	tagID, _, err := f.r.ODB.Write(&objects.Tag{Object: f.c3, Kind: objects.TypeCommit, Name: "v1", Tagger: who, Message: "m\n"})
	require.NoError(t, err)
	require.NoError(t, f.r.Refs.Update("refs/tags/v1", tagID))

	got, err := revparse.Resolve(f.r, "v1")
	require.NoError(t, err)
	assert.Equal(t, tagID, got, "plain name stays the tag object")

	got, err = revparse.ResolveCommit(f.r, "v1")
	require.NoError(t, err)
	assert.Equal(t, f.c3, got)

	got, err = revparse.Resolve(f.r, "v1~1")
	require.NoError(t, err)
	assert.Equal(t, f.c2, got)
}

func TestResolveCommitRejectsTrees(t *testing.T) {
	f := newFixture(t)
	tree, _, _ := f.r.ODB.Write(&objects.Tree{})
	_, err := revparse.ResolveCommit(f.r, tree.String())
	assert.ErrorContains(t, err, "is a tree, not a commit")
}

func TestAmbiguousShortID(t *testing.T) {
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	seen := map[string]bool{}
	for i := 0; ; i++ {
		id, _, err := r.ODB.Write(&objects.Blob{Data: []byte{byte(i), byte(i >> 8)}})
		require.NoError(t, err)
		if seen[id.String()[:4]] {
			_, err := revparse.Resolve(r, id.String()[:4])
			var amb *odb.AmbiguousError
			assert.True(t, errors.As(err, &amb))
			return
		}
		seen[id.String()[:4]] = true
	}
}

func TestResolveStashEntries(t *testing.T) {
	f := newFixture(t)
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(1, 0).In(time.UTC)}
	require.NoError(t, f.r.Refs.AppendLog("refs/stash", refs.LogEntry{New: f.c1, Who: who, Message: "old"}))
	require.NoError(t, f.r.Refs.AppendLog("refs/stash", refs.LogEntry{Old: f.c1, New: f.c2, Who: who, Message: "new"}))

	got, err := revparse.Resolve(f.r, "stash@{0}")
	require.NoError(t, err)
	assert.Equal(t, f.c2, got)
	got, err = revparse.Resolve(f.r, "stash@{1}")
	require.NoError(t, err)
	assert.Equal(t, f.c1, got)
	_, err = revparse.Resolve(f.r, "stash@{2}")
	assert.ErrorContains(t, err, "refs/stash has only 2 reflog entries")
}

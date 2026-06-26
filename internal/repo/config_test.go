package repo_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func newRepo(t *testing.T) *repo.Repo {
	t.Helper()
	r, err := repo.Init(t.TempDir(), "main")
	require.NoError(t, err)
	return r
}

func TestConfigSetAndGet(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.ConfigSet("user.name", "Ada Lovelace"))
	require.NoError(t, r.ConfigSet("user.email", "ada@example.com"))

	v, ok, err := r.ConfigGet("user.name")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Ada Lovelace", v)

	_, ok, err = r.ConfigGet("user.signingkey")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConfigSetReplacesValueAndKeepsOtherSections(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.ConfigSet("user.name", "A"))
	require.NoError(t, r.ConfigSet("user.name", "B"))
	v, _, _ := r.ConfigGet("user.name")
	assert.Equal(t, "B", v)
	core, ok, _ := r.ConfigGet("core.repositoryformatversion")
	assert.True(t, ok)
	assert.Equal(t, "0", core)

	data, _ := os.ReadFile(r.GitDir + "/config")
	assert.Equal(t, 1, strings.Count(string(data), "name = "))
}

func TestConfigAddsKeyToExistingSection(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.ConfigSet("core.editor", "vi"))
	v, ok, _ := r.ConfigGet("core.editor")
	assert.True(t, ok)
	assert.Equal(t, "vi", v)
	bare, _, _ := r.ConfigGet("core.bare")
	assert.Equal(t, "false", bare)
}

func TestConfigRejectsBadKeys(t *testing.T) {
	r := newRepo(t)
	for _, k := range []string{"", "name", "a.b.c", ".x", "x."} {
		assert.Error(t, r.ConfigSet(k, "v"), k)
		_, _, err := r.ConfigGet(k)
		assert.Error(t, err, k)
	}
}

func TestIdentityFromConfig(t *testing.T) {
	r := newRepo(t)
	for _, k := range []string{"NAME", "EMAIL", "DATE"} {
		t.Setenv("GIT_AUTHOR_"+k, "")
	}
	_, err := r.Identity("author")
	assert.True(t, errors.Is(err, repo.ErrNoIdentity))

	require.NoError(t, r.ConfigSet("user.name", "Ada"))
	require.NoError(t, r.ConfigSet("user.email", "ada@example.com"))
	who, err := r.Identity("author")
	require.NoError(t, err)
	assert.Equal(t, "Ada", who.Name)
	assert.WithinDuration(t, time.Now(), who.When, 5e9)
}

func TestIdentityFromEnvironment(t *testing.T) {
	r := newRepo(t)
	t.Setenv("GIT_COMMITTER_NAME", "Grace")
	t.Setenv("GIT_COMMITTER_EMAIL", "grace@example.com")
	t.Setenv("GIT_COMMITTER_DATE", "1700000000 +0200")
	who, err := r.Identity("committer")
	require.NoError(t, err)
	assert.Equal(t, "Grace <grace@example.com> 1700000000 +0200", who.String())
}

func TestIdentityBadDate(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, r.ConfigSet("user.name", "Ada"))
	require.NoError(t, r.ConfigSet("user.email", "a@b.c"))
	t.Setenv("GIT_AUTHOR_DATE", "yesterday")
	_, err := r.Identity("author")
	assert.ErrorContains(t, err, "cannot parse date")
}

func TestParseDate(t *testing.T) {
	for in, want := range map[string]string{
		"1700000000 +0200":          "1700000000 +0200",
		"1700000000 -0530":          "1700000000 -0530",
		"2023-11-14T22:13:20+00:00": "1700000000 +0000",
		"2023-11-15 00:13:20 +0200": "1700000000 +0200",
	} {
		got, err := repo.ParseDate(in)
		require.NoError(t, err, in)
		who := objectsSig(got)
		assert.Equal(t, "X <x@y> "+want, who, in)
	}
}

func objectsSig(t time.Time) string {
	return objects.Signature{Name: "X", Email: "x@y", When: t}.String()
}

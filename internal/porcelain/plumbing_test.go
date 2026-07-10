package porcelain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func TestCatFileOfBlobTreeAndCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "hello.txt", "hello\n")
	write(t, r, "src/main.go", "package main\n")
	commitAll(t, r, "first")

	out, err := porcelain.CatFile(r, "ce01362", porcelain.CatPretty)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", out)

	out, err = porcelain.CatFile(r, "ce01362", porcelain.CatType)
	require.NoError(t, err)
	assert.Equal(t, "blob\n", out)
	out, err = porcelain.CatFile(r, "ce01362", porcelain.CatSize)
	require.NoError(t, err)
	assert.Equal(t, "6\n", out)

	out, err = porcelain.CatFile(r, "HEAD^{tree}", porcelain.CatPretty)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "100644 blob ce013625030ba8dba906f756967f9e9ca394464a\thello.txt")
	assert.Contains(t, lines[1], "040000 tree ")

	out, err = porcelain.CatFile(r, "HEAD", porcelain.CatPretty)
	require.NoError(t, err)
	assert.Contains(t, out, "author Ada Lovelace <ada@example.com>")
	assert.True(t, strings.HasSuffix(out, "\n\nfirst\n"))
}

func TestCatFileErrors(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1")
	commitAll(t, r, "one")
	_, err := porcelain.CatFile(r, "nope", porcelain.CatType)
	assert.Error(t, err)
	_, err = porcelain.CatFile(r, "HEAD", "-x")
	assert.ErrorContains(t, err, "unknown cat-file mode")
}

func TestHashObject(t *testing.T) {
	r := newRepo(t)
	id, err := porcelain.HashObject(r, []byte("hello\n"), false)
	require.NoError(t, err)
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a", id.String())
	assert.False(t, r.ODB.Has(id))

	_, err = porcelain.HashObject(r, []byte("hello\n"), true)
	require.NoError(t, err)
	assert.True(t, r.ODB.Has(id))
}

func TestLsFiles(t *testing.T) {
	r := newRepo(t)
	write(t, r, "b.txt", "hello\n")
	write(t, r, "a/c.txt", "hello\n")
	_, err := porcelain.Add(r, []string{"."})
	require.NoError(t, err)

	plain, err := porcelain.LsFiles(r, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"a/c.txt", "b.txt"}, plain)

	staged, err := porcelain.LsFiles(r, true)
	require.NoError(t, err)
	assert.Equal(t, "100644 ce013625030ba8dba906f756967f9e9ca394464a 0\tb.txt", staged[1])
}

// Real git must read the repository we built and agree with us.
func TestRealGitReadsOurRepository(t *testing.T) {
	testutil.RequireGit(t)
	r := newRepo(t)
	write(t, r, "hello.txt", "hello\n")
	write(t, r, "src/main.go", "package main\n")
	write(t, r, "src-extra.txt", "x\n")
	commitAll(t, r, "first")
	tick(t)
	write(t, r, "hello.txt", "hello again\n")
	commitAll(t, r, "second")

	git := func(args ...string) string {
		return testutil.Git(t, r.Root, "", append([]string{"--git-dir=" + r.GitDir, "--work-tree=" + r.Root}, args...)...)
	}
	git("fsck", "--strict")
	assert.Equal(t, "", git("status", "--porcelain", "--untracked-files=no"))
	assert.Equal(t, "second\nfirst\n", git("log", "--format=%s"))
	head, _ := r.Refs.Head()
	assert.Equal(t, head.ID.String(), strings.TrimSpace(git("rev-parse", "HEAD")))
	assert.Equal(t, "main", strings.TrimSpace(git("rev-parse", "--abbrev-ref", "HEAD")))
}

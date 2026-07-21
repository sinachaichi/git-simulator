package porcelain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func paths(files []diff.File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestDiffWorkingTreeAgainstIndex(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "one\n")
	write(t, r, "b", "x\n")
	commitAll(t, r, "first")

	got, err := porcelain.Diff(r, porcelain.DiffOptions{})
	require.NoError(t, err)
	assert.Empty(t, got)

	write(t, r, "a", "two\n")
	require.NoError(t, r.Work.Remove("b"))
	write(t, r, "untracked", "u\n")
	got, err = porcelain.Diff(r, porcelain.DiffOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, paths(got))
	assert.Equal(t, []byte("one\n"), got[0].Old)
	assert.Equal(t, []byte("two\n"), got[0].New)
	assert.Zero(t, got[1].NewMode, "b is deleted")
}

func TestDiffStagedAndBeforeFirstCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "one\n")
	_, err := porcelain.Add(r, []string{"a"})
	require.NoError(t, err)

	got, err := porcelain.Diff(r, porcelain.DiffOptions{Staged: true})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].OldMode, "new file")

	got, err = porcelain.Diff(r, porcelain.DiffOptions{})
	require.NoError(t, err)
	assert.Empty(t, got, "everything is staged, nothing is unstaged")
}

func TestDiffTwoRevisions(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	write(t, r, "a", "2\n")
	write(t, r, "new", "n\n")
	commitAll(t, r, "two")

	got, err := porcelain.Diff(r, porcelain.DiffOptions{Revs: []string{"HEAD~1", "HEAD"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "new"}, paths(got))

	got, err = porcelain.Diff(r, porcelain.DiffOptions{Revs: []string{"HEAD", "HEAD~1"}})
	require.NoError(t, err)
	assert.Equal(t, []byte("2\n"), got[0].Old, "reversed order swaps the sides")
}

func TestDiffRevisionAgainstWorkingTree(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	write(t, r, "a", "2\n")
	got, err := porcelain.Diff(r, porcelain.DiffOptions{Revs: []string{"HEAD"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, paths(got))
}

func TestDiffPathFilterAndErrors(t *testing.T) {
	r := newRepo(t)
	write(t, r, "src/a", "1\n")
	write(t, r, "doc/b", "1\n")
	commitAll(t, r, "one")
	write(t, r, "src/a", "2\n")
	write(t, r, "doc/b", "2\n")

	got, err := porcelain.Diff(r, porcelain.DiffOptions{Paths: []string{"src"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"src/a"}, paths(got))

	_, err = porcelain.Diff(r, porcelain.DiffOptions{Revs: []string{"a", "b", "c"}})
	assert.ErrorContains(t, err, "at most two")
	_, err = porcelain.Diff(r, porcelain.DiffOptions{Staged: true, Revs: []string{"HEAD", "HEAD"}})
	assert.ErrorContains(t, err, "--staged")
	_, err = porcelain.Diff(r, porcelain.DiffOptions{Revs: []string{"nope"}})
	assert.Error(t, err)
}

// The rendered patch must equal real git's output for the same repo.
func TestDiffMatchesRealGit(t *testing.T) {
	testutil.RequireGit(t)
	r := newRepo(t)
	write(t, r, "a.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\n")
	write(t, r, "gone.txt", "bye\n")
	commitAll(t, r, "first")
	write(t, r, "a.txt", "one\ntwo\nTHREE\nfour\nfive\nsix\nseven\neight\nnine\n")
	require.NoError(t, r.Work.Remove("gone.txt"))
	write(t, r, "new.txt", "fresh\n")
	_, err := porcelain.Add(r, []string{"new.txt"})
	require.NoError(t, err)

	git := func(args ...string) string {
		return testutil.Git(t, r.Root, "", append([]string{"--git-dir=" + r.GitDir, "--work-tree=" + r.Root}, args...)...)
	}
	short := func(id objects.ID) string { return id.String()[:7] }
	for _, staged := range []bool{false, true} {
		files, err := porcelain.Diff(r, porcelain.DiffOptions{Staged: staged})
		require.NoError(t, err)
		var ours strings.Builder
		for _, f := range files {
			ours.WriteString(f.Render(short, 3))
		}
		args := []string{"diff", "--no-color"}
		if staged {
			args = append(args, "--staged")
		}
		assert.NotEmpty(t, ours.String())
		assert.Equal(t, git(args...), ours.String(), "staged=%v", staged)
	}
}

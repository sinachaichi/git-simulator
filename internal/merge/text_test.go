package merge_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

var labels = merge.Labels{Ours: "HEAD", Theirs: "feature"}

func lines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestTextTakesChangesFromEitherSide(t *testing.T) {
	base := lines(10)
	ours := strings.Replace(base, "line 2\n", "ours 2\n", 1)
	theirs := strings.Replace(base, "line 8\n", "theirs 8\n", 1)
	got, n := merge.Text(base, ours, theirs, labels)
	assert.Zero(t, n)
	assert.Equal(t, strings.Replace(ours, "line 8\n", "theirs 8\n", 1), got)
}

func TestTextSameChangeOnBothSidesIsNotAConflict(t *testing.T) {
	base := lines(5)
	both := strings.Replace(base, "line 3\n", "same\n", 1)
	got, n := merge.Text(base, both, both, labels)
	assert.Zero(t, n)
	assert.Equal(t, both, got)
}

func TestTextOneSideUnchangedTakesTheOther(t *testing.T) {
	base := lines(5)
	changed := lines(7)
	got, n := merge.Text(base, base, changed, labels)
	assert.Zero(t, n)
	assert.Equal(t, changed, got)
	got, n = merge.Text(base, changed, base, labels)
	assert.Zero(t, n)
	assert.Equal(t, changed, got)
}

func TestTextConflictMarkers(t *testing.T) {
	base := lines(5)
	ours := strings.Replace(base, "line 3\n", "ours\n", 1)
	theirs := strings.Replace(base, "line 3\n", "theirs\n", 1)
	got, n := merge.Text(base, ours, theirs, labels)
	assert.Equal(t, 1, n)
	assert.Equal(t, "line 1\nline 2\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\nline 4\nline 5\n", got)
}

func TestTextAdjacentChangesConflict(t *testing.T) {
	base := "a\nb\nc\n"
	got, n := merge.Text(base, "A\nb\nc\n", "a\nB\nc\n", labels)
	assert.Equal(t, 1, n)
	assert.Contains(t, got, "<<<<<<< HEAD\n")
}

func TestTextDeleteVersusEditConflicts(t *testing.T) {
	base := "a\nb\nc\n"
	got, n := merge.Text(base, "a\nc\n", "a\nB\nc\n", labels)
	assert.Equal(t, 1, n)
	assert.Equal(t, "a\n<<<<<<< HEAD\n=======\nB\n>>>>>>> feature\nc\n", got)
}

func TestTextEmptyBaseBothAdd(t *testing.T) {
	got, n := merge.Text("", "ours\n", "theirs\n", labels)
	assert.Equal(t, 1, n)
	assert.Equal(t, "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n", got)
	got, n = merge.Text("", "same\n", "same\n", labels)
	assert.Zero(t, n)
	assert.Equal(t, "same\n", got)
}

func TestTextMissingFinalNewlineStillStartsMarkersOnNewLine(t *testing.T) {
	got, n := merge.Text("a\n", "a\nours", "a\ntheirs", labels)
	assert.Equal(t, 1, n)
	assert.Equal(t, "a\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n", got)
}

func TestTextTwoSeparateConflicts(t *testing.T) {
	base := lines(12)
	ours := strings.Replace(strings.Replace(base, "line 2\n", "o2\n", 1), "line 10\n", "o10\n", 1)
	theirs := strings.Replace(strings.Replace(base, "line 2\n", "t2\n", 1), "line 10\n", "t10\n", 1)
	_, n := merge.Text(base, ours, theirs, labels)
	assert.Equal(t, 2, n)
}

// For these inputs real git produces the same text and conflict count.
func TestTextMatchesGitMergeFile(t *testing.T) {
	testutil.RequireGit(t)
	cases := map[string][3]string{ // base, ours, theirs
		"clean, far apart": {lines(10), strings.Replace(lines(10), "line 2\n", "o\n", 1), strings.Replace(lines(10), "line 9\n", "t\n", 1)},
		"conflict":         {lines(6), strings.Replace(lines(6), "line 3\n", "o\n", 1), strings.Replace(lines(6), "line 3\n", "t\n", 1)},
		"both add at end":  {lines(3), lines(3) + "ours\n", lines(3) + "theirs\n"},
		"insert vs insert": {lines(6), strings.Replace(lines(6), "line 3\n", "line 3\nours\n", 1), strings.Replace(lines(6), "line 4\n", "line 4\ntheirs\n", 1)},
		"delete vs edit":   {"a\nb\nc\n", "a\nc\n", "a\nB\nc\n"},
		"identical change": {lines(4), strings.Replace(lines(4), "line 2\n", "x\n", 1), strings.Replace(lines(4), "line 2\n", "x\n", 1)},
		"one side only":    {lines(4), lines(4), lines(6)},
		"two conflicts": {
			lines(12),
			strings.Replace(strings.Replace(lines(12), "line 2\n", "o2\n", 1), "line 10\n", "o10\n", 1),
			strings.Replace(strings.Replace(lines(12), "line 2\n", "t2\n", 1), "line 10\n", "t10\n", 1),
		},
		"common prefix in conflict": {"a\nb\nc\n", "a\nX\nY\nc\n", "a\nX\nZ\nc\n"},
	}
	for name, c := range cases {
		dir := t.TempDir()
		for file, content := range map[string]string{"base": c[0], "ours": c[1], "theirs": c[2]} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644))
		}
		want, code := testutil.GitExit(t, dir, "merge-file", "-p", "-L", "HEAD", "-L", "base", "-L", "feature", "ours", "base", "theirs")
		got, n := merge.Text(c[0], c[1], c[2], labels)
		assert.Equal(t, want, got, name)
		assert.Equal(t, code, n, name)
	}
}

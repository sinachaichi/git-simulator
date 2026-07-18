package diff_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func render(a, b string, ctx int) string {
	var out strings.Builder
	for _, h := range diff.Hunks(diff.Lines(diff.SplitLines(a), diff.SplitLines(b)), ctx) {
		out.WriteString(h.String())
	}
	return out.String()
}

func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestNoChangesNoHunks(t *testing.T) {
	assert.Empty(t, diff.Hunks(diff.Lines(diff.SplitLines("a\n"), diff.SplitLines("a\n")), 3))
}

func TestSingleChangeWithContext(t *testing.T) {
	old := numbered(10)
	updated := strings.Replace(old, "line 5\n", "five\n", 1)
	assert.Equal(t, "@@ -2,7 +2,7 @@ line 1\n line 2\n line 3\n line 4\n-line 5\n+five\n line 6\n line 7\n line 8\n", render(old, updated, 3))
}

func TestFarApartChangesMakeTwoHunks(t *testing.T) {
	old := numbered(30)
	updated := strings.Replace(strings.Replace(old, "line 3\n", "x\n", 1), "line 28\n", "y\n", 1)
	hunks := diff.Hunks(diff.Lines(diff.SplitLines(old), diff.SplitLines(updated)), 3)
	require.Len(t, hunks, 2)
	assert.Equal(t, "@@ -1,6 +1,6 @@", hunks[0].Header())
	assert.Equal(t, "@@ -25,6 +25,6 @@ line 24", hunks[1].Header())
}

func TestCloseChangesShareOneHunk(t *testing.T) {
	old := numbered(20)
	updated := strings.Replace(strings.Replace(old, "line 5\n", "x\n", 1), "line 11\n", "y\n", 1)
	// 5 unchanged lines between the changes, which is less than 2*3.
	assert.Len(t, diff.Hunks(diff.Lines(diff.SplitLines(old), diff.SplitLines(updated)), 3), 1)
}

func TestHeaderOmitsCountOfOneAndHandlesEmptySides(t *testing.T) {
	assert.Equal(t, "@@ -0,0 +1 @@\n+hello\n", render("", "hello\n", 3))
	assert.Equal(t, "@@ -1 +0,0 @@\n-hello\n", render("hello\n", "", 3))
}

func TestNoNewlineAtEndOfFile(t *testing.T) {
	assert.Equal(t, "@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+a\n", render("a", "a\n", 3))
}

func TestSectionHeadingUsesLastFunctionLikeLine(t *testing.T) {
	old := "func main() {\n\tone()\n\ttwo()\n\tthree()\n\tfour()\n\tfive()\n}\n"
	updated := strings.Replace(old, "five()", "FIVE()", 1)
	hunks := diff.Hunks(diff.Lines(diff.SplitLines(old), diff.SplitLines(updated)), 1)
	require.Len(t, hunks, 1)
	assert.Equal(t, "@@ -5,3 +5,3 @@ func main() {", hunks[0].Header())
}

// Our hunks must match `git diff --no-index` line for line.
func TestHunksMatchRealGit(t *testing.T) {
	testutil.RequireGit(t)
	cases := map[string][2]string{
		"change in the middle": {numbered(20), strings.Replace(numbered(20), "line 10\n", "ten\n", 1)},
		"add at the end":       {numbered(5), numbered(7)},
		"delete at the start":  {numbered(8), strings.TrimPrefix(numbered(8), "line 1\n")},
		"two far changes":      {numbered(40), strings.Replace(strings.Replace(numbered(40), "line 2\n", "a\n", 1), "line 38\n", "b\n", 1)},
		"no newline":           {"a\nb", "a\nc"},
		"whole file new":       {"", numbered(3)},
		"whole file removed":   {numbered(3), ""},
		"insert two apart":     {numbered(12), strings.Replace(strings.Replace(numbered(12), "line 4\n", "line 4\nnew1\n", 1), "line 9\n", "line 9\nnew2\n", 1)},
		"function heading":     {"func a() {\n\tx\n\ty\n\tz\n\tw\n\tv\n\tu\n}\n", "func a() {\n\tx\n\ty\n\tz\n\tw\n\tv\n\tU\n}\n"},
	}
	for name, c := range cases {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte(c[0]), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "b"), []byte(c[1]), 0o644))
		out := gitNoIndex(t, dir)
		_, want, _ := strings.Cut(out, "@@")
		if want != "" {
			want = "@@" + want
		}
		assert.Equal(t, want, render(c[0], c[1], 3), name)
	}
}

// gitNoIndex runs git diff --no-index, which exits 1 when files differ.
func gitNoIndex(t *testing.T, dir string) string {
	t.Helper()
	return testutil.GitAllowDiff(t, dir, "diff", "--no-index", "--no-color", "a", "b")
}

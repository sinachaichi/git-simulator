package diff_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/diff"
)

// apply rebuilds the new text from an edit script, which proves the
// script is a valid description of the change.
func apply(edits []diff.Edit) (oldText, newText string) {
	var o, n strings.Builder
	for _, e := range edits {
		if e.Kind != diff.Insert {
			o.WriteString(e.Line)
		}
		if e.Kind != diff.Delete {
			n.WriteString(e.Line)
		}
	}
	return o.String(), n.String()
}

func TestSplitLines(t *testing.T) {
	assert.Nil(t, diff.SplitLines(""))
	assert.Equal(t, []string{"a\n", "b\n"}, diff.SplitLines("a\nb\n"))
	assert.Equal(t, []string{"a\n", "b"}, diff.SplitLines("a\nb"))
	assert.Equal(t, []string{"\n", "\n"}, diff.SplitLines("\n\n"))
}

func TestLinesIdenticalAndEmpty(t *testing.T) {
	assert.Nil(t, diff.Lines(nil, nil))
	edits := diff.Lines([]string{"a\n", "b\n"}, []string{"a\n", "b\n"})
	require.Len(t, edits, 2)
	for _, e := range edits {
		assert.Equal(t, diff.Equal, e.Kind)
	}
}

func TestLinesPureInsertAndDelete(t *testing.T) {
	edits := diff.Lines(nil, []string{"a\n", "b\n"})
	assert.Equal(t, []diff.Edit{
		{Kind: diff.Insert, Line: "a\n", NewLine: 1},
		{Kind: diff.Insert, Line: "b\n", NewLine: 2},
	}, edits)
	edits = diff.Lines([]string{"a\n"}, nil)
	assert.Equal(t, []diff.Edit{{Kind: diff.Delete, Line: "a\n", OldLine: 1}}, edits)
}

// The example from the Myers paper: ABCABBA to CBABAC has an edit
// distance of 5.
func TestLinesMyersPaperExample(t *testing.T) {
	a := strings.Split("ABCABBA", "")
	b := strings.Split("CBABAC", "")
	edits := diff.Lines(a, b)
	changes := 0
	for _, e := range edits {
		if e.Kind != diff.Equal {
			changes++
		}
	}
	assert.Equal(t, 5, changes)
	o, n := apply(edits)
	assert.Equal(t, "ABCABBA", o)
	assert.Equal(t, "CBABAC", n)
}

func TestLinesLineNumbers(t *testing.T) {
	edits := diff.Lines([]string{"a\n", "b\n", "c\n"}, []string{"a\n", "x\n", "c\n"})
	assert.Equal(t, []diff.Edit{
		{Kind: diff.Equal, Line: "a\n", OldLine: 1, NewLine: 1},
		{Kind: diff.Delete, Line: "b\n", OldLine: 2},
		{Kind: diff.Insert, Line: "x\n", NewLine: 2},
		{Kind: diff.Equal, Line: "c\n", OldLine: 3, NewLine: 3},
	}, edits)
}

func TestLinesDistinguishesMissingFinalNewline(t *testing.T) {
	edits := diff.Lines(diff.SplitLines("a\nb"), diff.SplitLines("a\nb\n"))
	kinds := []diff.Kind{}
	for _, e := range edits {
		kinds = append(kinds, e.Kind)
	}
	assert.Equal(t, []diff.Kind{diff.Equal, diff.Delete, diff.Insert}, kinds)
}

func TestLinesRandomInputsRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	words := []string{"a\n", "b\n", "c\n", "d\n", "e\n"}
	pick := func() []string {
		out := make([]string, rng.Intn(25))
		for i := range out {
			out[i] = words[rng.Intn(len(words))]
		}
		return out
	}
	for i := 0; i < 300; i++ {
		a, b := pick(), pick()
		o, n := apply(diff.Lines(a, b))
		require.Equal(t, strings.Join(a, ""), o)
		require.Equal(t, strings.Join(b, ""), n)
	}
}

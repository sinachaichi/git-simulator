package graph_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/graph"
	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/term"
)

var clock int64 = 1700000000

// node builds an in-memory commit. Ids come from the content, so the
// message must be unique.
func node(msg string, parents ...history.Node) history.Node {
	clock += 60
	who := objects.Signature{Name: "A", Email: "a@b.c", When: time.Unix(clock, 0).In(time.UTC)}
	c := &objects.Commit{Author: who, Committer: who, Message: msg + "\n"}
	for _, p := range parents {
		c.Parents = append(c.Parents, p.ID)
	}
	return history.Node{ID: objects.HashOf(c), Commit: c}
}

func draw(nodes ...history.Node) string {
	rows := graph.Layout(graph.TopoOrder(nodes))
	lines := graph.Render(term.Style{}, rows, func(n history.Node) string { return n.Commit.Subject() })
	return strings.Join(lines, "\n") + "\n"
}

func TestLinearHistory(t *testing.T) {
	a := node("a")
	b := node("b", a)
	c := node("c", b)
	assert.Equal(t, "* c\n* b\n* a\n", draw(c, b, a))
}

func TestBranchAndMerge(t *testing.T) {
	base := node("base")
	main1 := node("main1", base)
	feat1 := node("feat1", base)
	merge := node("merge", main1, feat1)
	want := "*   merge\n|\\\n| * feat1\n* | main1\n|/\n* base\n"
	assert.Equal(t, want, draw(merge, feat1, main1, base))
}

func TestTwoTipsWithoutMerge(t *testing.T) {
	base := node("base")
	one := node("one", base)
	two := node("two", base)
	want := "* two\n| * one\n|/\n* base\n"
	assert.Equal(t, want, draw(two, one, base))
}

func TestTopoOrderPutsChildrenBeforeParentsEvenWithSkewedClocks(t *testing.T) {
	parent := node("parent")
	child := node("child", parent)
	// Pretend the child was made at an earlier time than its parent.
	child.Commit.Committer.When = parent.Commit.Committer.When.Add(-time.Hour)
	got := graph.TopoOrder([]history.Node{parent, child})
	assert.Equal(t, "child", got[0].Commit.Subject())
	assert.Equal(t, "parent", got[1].Commit.Subject())
}

func TestTopoOrderShowsTheLastParentFirst(t *testing.T) {
	base := node("base")
	first := node("first parent", base)
	second := node("second parent", base)
	merge := node("merge", first, second)
	// Date order would put "second parent" first anyway, so make the first
	// parent the newest commit to see that the stack decides.
	first.Commit.Committer.When = second.Commit.Committer.When.Add(time.Hour)
	got := graph.TopoOrder([]history.Node{merge, first, second, base})
	var subjects []string
	for _, n := range got {
		subjects = append(subjects, n.Commit.Subject())
	}
	assert.Equal(t, []string{"merge", "second parent", "first parent", "base"}, subjects)
}

func TestColorsOnlyWhenEnabled(t *testing.T) {
	a := node("a")
	b := node("b", a)
	rows := graph.Layout(graph.TopoOrder([]history.Node{b, a}))
	plain := graph.Text(term.Style{}, rows[0].Prefix)
	colored := graph.Text(term.Style{Enabled: true}, rows[0].Prefix)
	assert.Equal(t, "* ", plain)
	assert.Contains(t, colored, "\x1b[")
}

func TestOctopusMergeGetsALanePerParent(t *testing.T) {
	base := node("base")
	a := node("a", base)
	b := node("b", base)
	c := node("c", base)
	m := node("octopus", a, b, c)
	out := draw(m, a, b, c, base)
	require.Contains(t, out, "octopus")
	assert.Equal(t, 3, strings.Count(strings.Split(out, "\n")[1], "|")+strings.Count(strings.Split(out, "\n")[1], "\\"))
}

// This is the output of `git log --graph` for the same history. It has a
// merge next to another lane, a lane that joins and a "/" in a commit row.
func TestTwoMergesInARowLikeGit(t *testing.T) {
	base := node("base")
	one := node("one work", base)
	two := node("two work", base)
	mainWork := node("main work", base)
	m1 := node("merge one", mainWork, one)
	m2 := node("merge two", m1, two)
	want := `*   merge two
|\
| * two work
* |   merge one
|\ \
| * | one work
| |/
* / main work
|/
* base
`
	assert.Equal(t, want, draw(m2, m1, mainWork, two, one, base))
}

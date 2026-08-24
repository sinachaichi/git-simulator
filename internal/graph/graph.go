// Package graph draws the commit graph with ASCII lanes, in the style of
// `git log --graph`.
package graph

import (
	"strings"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/term"
)

// Row is one commit with the graph lines that go with it.
type Row struct {
	Node history.Node
	// Prefix is the graph part in front of the commit text, such as "| * ".
	Prefix []Cell
	// Between holds the connector lines drawn below the commit, such as
	// "|\" for a merge or "|/" where two lanes join.
	Between [][]Cell
}

// Cell is one character of the graph and the lane it belongs to. The
// color is the lane number, and a negative value means no color.
type Cell struct {
	Ch    byte
	Color int
}

type lane struct {
	id    objects.ID
	color int
}

// TopoOrder reorders commits so that every commit comes before its
// parents. It follows git's graph order: a stack of ready commits where
// the last parent of a merge is shown first, so one line of history is
// finished before the next one starts. nodes should be in date order,
// newest first, which decides the order of unrelated tips.
func TopoOrder(nodes []history.Node) []history.Node {
	index := make(map[objects.ID]int, len(nodes))
	for i, n := range nodes {
		index[n.ID] = i
	}
	children := make([]int, len(nodes))
	for _, n := range nodes {
		for _, p := range n.Commit.Parents {
			if j, ok := index[p]; ok {
				children[j]++
			}
		}
	}
	var stack []int
	for i := len(nodes) - 1; i >= 0; i-- { // the newest tip ends up on top
		if children[i] == 0 {
			stack = append(stack, i)
		}
	}
	out := make([]history.Node, 0, len(nodes))
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, nodes[i])
		for _, p := range nodes[i].Commit.Parents {
			if j, ok := index[p]; ok {
				if children[j]--; children[j] == 0 {
					stack = append(stack, j)
				}
			}
		}
	}
	return out
}

// Layout assigns lanes to commits. nodes must be in TopoOrder.
func Layout(nodes []history.Node) []Row {
	var lanes []lane
	nextColor := 0
	rows := make([]Row, 0, len(nodes))
	afterCollapse := false // the line above the next commit joined two lanes

	for _, n := range nodes {
		col := -1
		for k, l := range lanes {
			if l.id == n.ID {
				col = k
				break
			}
		}
		if col < 0 {
			lanes = append(lanes, lane{id: n.ID, color: nextColor})
			nextColor++
			col = len(lanes) - 1
		}

		// The first parent keeps the lane of this commit. Other parents that
		// no lane waits for yet get a new lane to the right of it.
		parents := n.Commit.Parents
		var added []lane
		for _, p := range parents[min(1, len(parents)):] {
			if containsLane(lanes, p) || containsLane(added, p) {
				continue
			}
			added = append(added, lane{id: p, color: nextColor})
			nextColor++
		}

		row := Row{Node: n}
		row.Prefix = commitCells(lanes, col, len(added), firstParent(parents), afterCollapse)

		oldRight := len(lanes) - col - 1
		colColor := lanes[col].color
		if len(parents) == 0 {
			lanes = append(lanes[:col], lanes[col+1:]...)
		} else {
			lanes[col].id = parents[0]
		}
		if len(added) > 0 {
			at := col + 1
			lanes = append(lanes[:at], append(append([]lane(nil), added...), lanes[at:]...)...)
			row.Between = append(row.Between, mergeCells(lanes, col, colColor, added, oldRight))
			afterCollapse = false
		}

		// Two lanes that wait for the same commit become one.
		for j := 1; j < len(lanes); j++ {
			for i := 0; i < j; i++ {
				if lanes[i].id == lanes[j].id {
					row.Between = append(row.Between, collapseCells(lanes, j))
					afterCollapse = true
					lanes = append(lanes[:j], lanes[j+1:]...)
					j--
					break
				}
			}
		}
		if len(row.Between) == 0 {
			afterCollapse = false
		}
		rows = append(rows, row)
	}
	return rows
}

func containsLane(lanes []lane, id objects.ID) bool {
	for _, l := range lanes {
		if l.id == id {
			return true
		}
	}
	return false
}

// firstParent returns the first parent, or the zero id for a root commit.
func firstParent(parents []objects.ID) objects.ID {
	if len(parents) == 0 {
		return objects.Zero
	}
	return parents[0]
}

// commitCells draws the lanes for the commit row: a star in the commit's
// lane and a bar in the others. A lane to the right that waits for the
// same parent as this commit is about to join it. When the line above
// was already a join, git draws that lane as "/". A merge needs room for
// its new lanes, which is added at the end.
func commitCells(lanes []lane, col, extra int, parent objects.ID, afterCollapse bool) []Cell {
	var cells []Cell
	for k, l := range lanes {
		ch := byte('|')
		switch {
		case k == col:
			ch = '*'
		case afterCollapse && k > col && !parent.IsZero() && l.id == parent:
			ch = '/'
		}
		cells = append(cells, Cell{ch, l.color}, Cell{' ', -1})
	}
	for i := 0; i < extra; i++ {
		cells = append(cells, Cell{' ', -1}, Cell{' ', -1})
	}
	return cells
}

// mergeCells draws the line below a merge commit: the commit's lane, a
// "\" for each new lane, and "\" for the old lanes that moved right.
func mergeCells(lanes []lane, col, colColor int, added []lane, oldRight int) []Cell {
	var cells []Cell
	for k := 0; k < col; k++ {
		cells = append(cells, Cell{'|', lanes[k].color}, Cell{' ', -1})
	}
	cells = append(cells, Cell{'|', colColor})
	for _, l := range added {
		cells = append(cells, Cell{'\\', l.color})
	}
	for k := 0; k < oldRight; k++ {
		l := lanes[col+len(added)+1+k]
		cells = append(cells, Cell{' ', -1}, Cell{'\\', l.color})
	}
	return cells
}

// collapseCells draws the line where lane j joins an earlier lane. The
// lanes to its right move one step left and are drawn with "/".
func collapseCells(lanes []lane, j int) []Cell {
	var cells []Cell
	for k := 0; k < j-1; k++ {
		cells = append(cells, Cell{'|', lanes[k].color}, Cell{' ', -1})
	}
	cells = append(cells, Cell{'|', lanes[j-1].color}, Cell{'/', lanes[j].color})
	for k := j + 1; k < len(lanes); k++ {
		cells = append(cells, Cell{' ', -1}, Cell{'/', lanes[k].color})
	}
	return cells
}

// Continuation returns the lanes for the lines under a commit row: the
// same cells with the star turned into a bar and the extra space of a
// merge removed.
func Continuation(prefix []Cell) []Cell {
	out := make([]Cell, 0, len(prefix))
	for _, c := range prefix {
		if c.Ch == '*' {
			c.Ch = '|'
		}
		out = append(out, c)
	}
	return out
}

// palette gives each lane its own color, in the order git uses.
var palette = []func(term.Style, string) string{
	term.Style.Red, term.Style.Green, term.Style.Yellow,
	term.Style.Blue, term.Style.Purple, term.Style.Cyan,
}

// Text turns cells into a string, with colors when the style allows it.
func Text(s term.Style, cells []Cell) string {
	var b strings.Builder
	for _, c := range cells {
		ch := string(c.Ch)
		if c.Color >= 0 && c.Ch != ' ' {
			ch = palette[c.Color%len(palette)](s, ch)
		}
		b.WriteString(ch)
	}
	return b.String()
}

// Render draws the rows. label returns the text shown next to a commit,
// for example its short id, decorations and subject.
func Render(s term.Style, rows []Row, label func(history.Node) string) []string {
	var lines []string
	for _, r := range rows {
		lines = append(lines, Text(s, r.Prefix)+label(r.Node))
		for _, between := range r.Between {
			lines = append(lines, strings.TrimRight(Text(s, between), " "))
		}
	}
	return lines
}

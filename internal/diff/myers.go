// Package diff compares texts line by line and renders unified diffs the
// way git does.
package diff

import "strings"

// Kind says what an edit does to a line.
type Kind int

// The three kinds of edit.
const (
	Equal Kind = iota
	Insert
	Delete
)

// Edit is one line of an edit script. OldLine and NewLine are 1-based
// positions, or 0 when the line does not exist on that side.
type Edit struct {
	Kind    Kind
	Line    string
	OldLine int
	NewLine int
}

// SplitLines cuts text into lines and keeps each line terminator, so a
// last line without a newline differs from the same line with one.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Lines returns the shortest edit script from a to b, using Myers'
// O(ND) algorithm.
func Lines(a, b []string) []Edit {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	limit := n + m
	offset := limit
	v := make([]int, 2*limit+2)
	var trace [][]int
	for d := 0; d <= limit; d++ {
		snapshot := make([]int, len(v))
		copy(snapshot, v)
		trace = append(trace, snapshot)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return backtrack(a, b, trace, d, offset)
			}
		}
	}
	return nil
}

// backtrack walks the saved V arrays from the end to the start and
// rebuilds the edits in forward order.
func backtrack(a, b []string, trace [][]int, dEnd, offset int) []Edit {
	var rev []Edit
	x, y := len(a), len(b)
	for d := dEnd; d > 0; d-- {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			rev = append(rev, Edit{Kind: Equal, Line: a[x], OldLine: x + 1, NewLine: y + 1})
		}
		if x == prevX {
			y--
			rev = append(rev, Edit{Kind: Insert, Line: b[y], NewLine: y + 1})
		} else {
			x--
			rev = append(rev, Edit{Kind: Delete, Line: a[x], OldLine: x + 1})
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		rev = append(rev, Edit{Kind: Equal, Line: a[x], OldLine: x + 1, NewLine: y + 1})
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

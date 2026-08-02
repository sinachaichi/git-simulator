// Package merge implements three-way merging of texts and of trees.
package merge

import (
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
)

// Labels name the two sides in conflict markers.
type Labels struct {
	Ours   string
	Theirs string
}

// span is a region of the base text that one side replaced. For a pure
// insertion start equals end.
type span struct {
	start, end int
	lines      []string
}

// changes lists where side differs from base, in base order.
func changes(base, side []string) []span {
	var out []span
	pos := 0
	var cur *span
	for _, e := range diff.Lines(base, side) {
		switch e.Kind {
		case diff.Equal:
			if cur != nil {
				cur.end = pos
				out, cur = append(out, *cur), nil
			}
			pos++
		case diff.Delete:
			if cur == nil {
				cur = &span{start: pos}
			}
			pos++
		case diff.Insert:
			if cur == nil {
				cur = &span{start: pos}
			}
			cur.lines = append(cur.lines, e.Line)
		}
	}
	if cur != nil {
		cur.end = pos
		out = append(out, *cur)
	}
	return out
}

// Text merges two versions of a text that both come from base. It
// returns the merged text and the number of conflicts. Conflicts are
// written with <<<<<<<, ======= and >>>>>>> markers, like git does.
// Changes that touch each other, even without an overlap, conflict.
func Text(base, ours, theirs string, labels Labels) (string, int) {
	b, o, t := diff.SplitLines(base), diff.SplitLines(ours), diff.SplitLines(theirs)
	ro, rt := changes(b, o), changes(b, t)

	var out strings.Builder
	conflicts, pos, i, j := 0, 0, 0, 0
	for i < len(ro) || j < len(rt) {
		// Start a cluster with the earliest change, then pull in every
		// change from either side that touches it.
		var start int
		switch {
		case i >= len(ro):
			start = rt[j].start
		case j >= len(rt):
			start = ro[i].start
		default:
			start = min(ro[i].start, rt[j].start)
		}
		end := start
		var co, ct []span
		for {
			grew := false
			if i < len(ro) && ro[i].start <= end {
				co, end, i, grew = append(co, ro[i]), max(end, ro[i].end), i+1, true
			}
			if j < len(rt) && rt[j].start <= end {
				ct, end, j, grew = append(ct, rt[j]), max(end, rt[j].end), j+1, true
			}
			if !grew {
				break
			}
		}
		out.WriteString(strings.Join(b[pos:start], ""))
		pos = end

		baseSpan := b[start:end]
		oursText, theirsText := apply(b, start, end, co), apply(b, start, end, ct)
		switch {
		case equal(oursText, theirsText), equal(theirsText, baseSpan):
			out.WriteString(strings.Join(oursText, ""))
		case equal(oursText, baseSpan):
			out.WriteString(strings.Join(theirsText, ""))
		default:
			conflicts++
			writeConflict(&out, oursText, theirsText, labels)
		}
	}
	out.WriteString(strings.Join(b[pos:], ""))
	return out.String(), conflicts
}

// apply returns base[start:end] with the given changes of one side.
func apply(base []string, start, end int, spans []span) []string {
	var out []string
	pos := start
	for _, s := range spans {
		out = append(out, base[pos:s.start]...)
		out = append(out, s.lines...)
		pos = s.end
	}
	return append(out, base[pos:end]...)
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeConflict writes one conflict. Lines that both sides share at the
// start and the end are moved outside the markers, so the conflict is as
// small as it can be.
func writeConflict(out *strings.Builder, ours, theirs []string, labels Labels) {
	pre := 0
	for pre < len(ours) && pre < len(theirs) && ours[pre] == theirs[pre] {
		pre++
	}
	suf := 0
	for suf < len(ours)-pre && suf < len(theirs)-pre && ours[len(ours)-1-suf] == theirs[len(theirs)-1-suf] {
		suf++
	}
	out.WriteString(strings.Join(ours[:pre], ""))
	out.WriteString("<<<<<<< " + labels.Ours + "\n")
	writeLines(out, ours[pre:len(ours)-suf])
	out.WriteString("=======\n")
	writeLines(out, theirs[pre:len(theirs)-suf])
	out.WriteString(">>>>>>> " + labels.Theirs + "\n")
	out.WriteString(strings.Join(ours[len(ours)-suf:], ""))
}

// writeLines writes lines and makes sure the last one ends in a newline,
// so the next marker always starts on its own line.
func writeLines(out *strings.Builder, lines []string) {
	for _, l := range lines {
		out.WriteString(l)
	}
	if n := len(lines); n > 0 && !strings.HasSuffix(lines[n-1], "\n") {
		out.WriteByte('\n')
	}
}

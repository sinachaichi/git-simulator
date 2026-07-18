package diff

import (
	"fmt"
	"strings"
	"unicode"
)

// DefaultContext is the number of unchanged lines shown around a change.
const DefaultContext = 3

// Hunk is one "@@" block of a unified diff.
type Hunk struct {
	OldStart, OldLines int
	NewStart, NewLines int
	Section            string // text git shows after the second @@
	Edits              []Edit
}

// Hunks groups an edit script into hunks with the given amount of
// context. Two changes whose unchanged lines in between fit inside the
// context of both end up in the same hunk, as in git.
func Hunks(edits []Edit, context int) []Hunk {
	var changed []int
	for i, e := range edits {
		if e.Kind != Equal {
			changed = append(changed, i)
		}
	}
	var hunks []Hunk
	for i := 0; i < len(changed); {
		first, last := changed[i], changed[i]
		j := i + 1
		for j < len(changed) && changed[j]-last-1 <= 2*context {
			last = changed[j]
			j++
		}
		i = j
		start := max(0, first-context)
		end := min(len(edits), last+1+context)
		hunks = append(hunks, newHunk(edits, start, end))
	}
	return hunks
}

func newHunk(edits []Edit, start, end int) Hunk {
	h := Hunk{Edits: edits[start:end], Section: section(edits[:start])}
	oldBefore, newBefore := 0, 0
	for _, e := range edits[:start] {
		if e.Kind != Insert {
			oldBefore++
		}
		if e.Kind != Delete {
			newBefore++
		}
	}
	for _, e := range h.Edits {
		if e.Kind != Insert {
			h.OldLines++
		}
		if e.Kind != Delete {
			h.NewLines++
		}
	}
	h.OldStart, h.NewStart = oldBefore+1, newBefore+1
	if h.OldLines == 0 {
		h.OldStart = oldBefore
	}
	if h.NewLines == 0 {
		h.NewStart = newBefore
	}
	return h
}

// section finds the closest line above the hunk that looks like the
// start of a function. Git's default rule is a line that begins with a
// letter, underscore or dollar sign.
func section(before []Edit) string {
	for i := len(before) - 1; i >= 0; i-- {
		if before[i].Kind == Insert {
			continue
		}
		line := strings.TrimRight(before[i].Line, "\r\n")
		if line == "" {
			continue
		}
		if r := rune(line[0]); unicode.IsLetter(r) || r == '_' || r == '$' {
			if len(line) > 80 {
				line = line[:80]
			}
			return line
		}
	}
	return ""
}

// Header returns the "@@ -1,3 +1,4 @@" line.
func (h Hunk) Header() string {
	head := fmt.Sprintf("@@ -%s +%s @@", span(h.OldStart, h.OldLines), span(h.NewStart, h.NewLines))
	if h.Section != "" {
		head += " " + h.Section
	}
	return head
}

// span formats "start,count", leaving out the count when it is 1.
func span(start, count int) string {
	if count == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// String renders the hunk, header and lines.
func (h Hunk) String() string {
	var b strings.Builder
	b.WriteString(h.Header())
	b.WriteByte('\n')
	for _, e := range h.Edits {
		b.WriteByte(map[Kind]byte{Equal: ' ', Insert: '+', Delete: '-'}[e.Kind])
		b.WriteString(e.Line)
		if !strings.HasSuffix(e.Line, "\n") {
			b.WriteString("\n\\ No newline at end of file\n")
		}
	}
	return b.String()
}

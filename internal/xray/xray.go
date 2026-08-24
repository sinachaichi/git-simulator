// Package xray prints what a command did inside the repository: which
// objects it stored, which refs it moved, what changed in the index and
// in the working tree.
package xray

import (
	"fmt"
	"io"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/term"
)

type section struct {
	title string
	lines []string
}

// Render writes the events in groups. It writes nothing when there are
// no events.
func Render(w io.Writer, s term.Style, evs []events.Event) {
	sections := []*section{{title: "objects"}, {title: "index"}, {title: "refs"}, {title: "working tree"}, {title: "state"}}
	objs, idx, refs, work, state := sections[0], sections[1], sections[2], sections[3], sections[4]

	for _, e := range evs {
		switch e.Kind {
		case events.ObjectWritten:
			typ, size := splitDetail(e.Detail)
			objs.add(fmt.Sprintf("%s %s %s  %s", s.Green("+"), pad(typ, 6), s.Yellow(short(e.Subject)), s.Dim(size)))
		case events.ObjectExists:
			typ, _ := splitDetail(e.Detail)
			objs.add(fmt.Sprintf("%s %s %s  %s", s.Dim("="), pad(typ, 6), s.Yellow(short(e.Subject)), s.Dim("already stored, nothing written")))
		case events.IndexAdded:
			idx.add(fmt.Sprintf("%s %s  %s %s", s.Green("+"), e.Subject, e.Detail, s.Yellow(short(e.New))))
		case events.IndexModified:
			idx.add(fmt.Sprintf("%s %s  %s -> %s", s.Cyan("~"), e.Subject, s.Yellow(short(e.Old)), s.Yellow(short(e.New))))
		case events.IndexRemoved:
			idx.add(fmt.Sprintf("%s %s", s.Red("-"), e.Subject))
		case events.IndexConflict:
			idx.add(fmt.Sprintf("%s %s  %s", s.Red("!"), e.Subject, e.Detail))
		case events.RefUpdated:
			refs.add(fmt.Sprintf("%s  %s -> %s", e.Subject, s.Yellow(short(e.Old)), s.Yellow(short(e.New))))
		case events.RefDeleted:
			refs.add(fmt.Sprintf("%s  %s %s", e.Subject, s.Red("deleted"), s.Dim("(was "+short(e.Old)+")")))
		case events.HeadSet:
			if e.Detail == "symbolic" {
				refs.add(fmt.Sprintf("HEAD -> %s", e.New))
			} else {
				refs.add(fmt.Sprintf("HEAD detached at %s", s.Yellow(short(e.New))))
			}
		case events.ReflogAppended:
			refs.add(s.Dim(fmt.Sprintf("reflog %s: %s", e.Subject, e.Detail)))
		case events.FileWritten:
			work.add(fmt.Sprintf("%s wrote %s  %s", s.Green("+"), e.Subject, s.Dim(e.Detail)))
		case events.FileRemoved:
			work.add(fmt.Sprintf("%s removed %s", s.Red("-"), e.Subject))
		case events.StateWritten:
			state.add(fmt.Sprintf("%s written  %s", e.Subject, s.Dim(e.Detail)))
		case events.StateRemoved:
			state.add(fmt.Sprintf("%s removed", e.Subject))
		}
	}

	wrote := false
	for _, sec := range sections {
		if len(sec.lines) == 0 {
			continue
		}
		if !wrote {
			fmt.Fprintln(w, s.Bold(s.Purple("x-ray")))
			wrote = true
		}
		fmt.Fprintf(w, "  %s\n", s.Bold(sec.title))
		for _, l := range sec.lines {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
}

func (s *section) add(line string) { s.lines = append(s.lines, line) }

func short(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// splitDetail turns "blob 6 bytes" into "blob" and "6 bytes".
func splitDetail(d string) (typ, size string) {
	typ, size, _ = strings.Cut(d, " ")
	return typ, size
}

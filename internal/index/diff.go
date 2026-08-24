package index

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/events"
)

// Diff describes how the index changed from old to updated, as events:
// paths added, paths with new content or mode, paths removed, and paths
// that now have conflict stages.
func Diff(old, updated *Index) []events.Event {
	var out []events.Event
	before := map[string]Entry{}
	for _, e := range old.entries {
		if e.Stage == 0 {
			before[e.Path] = e
		}
	}
	conflicted := map[string]bool{}
	for _, p := range updated.Conflicts() {
		conflicted[p] = true
	}
	seen := map[string]bool{}
	for _, p := range updated.Paths() {
		seen[p] = true
		if conflicted[p] {
			out = append(out, events.Event{Kind: events.IndexConflict, Subject: p, Detail: stageList(updated.Stages(p))})
			continue
		}
		e, _ := updated.Get(p)
		prev, existed := before[p]
		switch {
		case !existed:
			out = append(out, events.Event{Kind: events.IndexAdded, Subject: p, New: e.ID.String(), Detail: fmt.Sprintf("%06o", e.Mode)})
		case prev.ID != e.ID || prev.Mode != e.Mode:
			out = append(out, events.Event{Kind: events.IndexModified, Subject: p, Old: prev.ID.String(), New: e.ID.String(), Detail: fmt.Sprintf("%06o", e.Mode)})
		}
	}
	for _, e := range old.entries {
		if e.Stage == 0 && !seen[e.Path] {
			out = append(out, events.Event{Kind: events.IndexRemoved, Subject: e.Path, Old: e.ID.String()})
		}
	}
	return out
}

func stageList(st [3]*Entry) string {
	names := []string{"base", "ours", "theirs"}
	var out string
	for i, e := range st {
		if e != nil {
			if out != "" {
				out += ", "
			}
			out += fmt.Sprintf("stage %d (%s)", i+1, names[i])
		}
	}
	return out
}

package index_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/index"
)

func TestDiffReportsAddedModifiedRemovedAndConflicts(t *testing.T) {
	old := index.New()
	old.Add(entry("same", "1"))
	old.Add(entry("changed", "1"))
	old.Add(entry("gone", "1"))

	updated := index.New()
	updated.Add(entry("same", "1"))
	updated.Add(entry("changed", "2"))
	updated.Add(entry("new", "n"))
	ours, theirs := entry("clash", "o"), entry("clash", "t")
	updated.SetConflict("clash", nil, &ours, &theirs)

	got := index.Diff(old, updated)
	kinds := map[string]events.Kind{}
	for _, e := range got {
		kinds[e.Subject] = e.Kind
	}
	assert.Equal(t, map[string]events.Kind{
		"changed": events.IndexModified,
		"new":     events.IndexAdded,
		"clash":   events.IndexConflict,
		"gone":    events.IndexRemoved,
	}, kinds)
	for _, e := range got {
		if e.Subject == "clash" {
			assert.Equal(t, "stage 2 (ours), stage 3 (theirs)", e.Detail)
		}
	}
}

func TestDiffOfEqualIndexesIsEmpty(t *testing.T) {
	a, b := index.New(), index.New()
	a.Add(entry("x", "1"))
	b.Add(entry("x", "1"))
	assert.Empty(t, index.Diff(a, b))
}

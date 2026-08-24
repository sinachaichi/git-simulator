package refs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
)

func TestRefChangesAreReported(t *testing.T) {
	s := newStore(t)
	var rec events.Recorder
	s.Observer = &rec

	require.NoError(t, s.SetHeadBranch("main"))
	require.NoError(t, s.Update("refs/heads/main", id("a")))
	require.NoError(t, s.Update("refs/heads/main", id("b")))
	require.NoError(t, s.SetHeadDetached(id("b")))
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{Old: id("a"), New: id("b"), Who: who(1), Message: "commit: two"}))
	require.NoError(t, s.Delete("refs/heads/main"))

	got := rec.Events()
	require.Len(t, got, 6)
	assert.Equal(t, events.Event{Kind: events.HeadSet, Subject: "HEAD", New: "refs/heads/main", Detail: "symbolic"}, got[0])
	assert.Equal(t, events.Event{Kind: events.RefUpdated, Subject: "refs/heads/main", Old: objects.Zero.String(), New: id("a").String()}, got[1])
	assert.Equal(t, id("a").String(), got[2].Old, "the second update knows where the ref was")
	assert.Equal(t, "detached", got[3].Detail)
	assert.Equal(t, events.ReflogAppended, got[4].Kind)
	assert.Equal(t, "commit: two", got[4].Detail)
	assert.Equal(t, events.Event{Kind: events.RefDeleted, Subject: "refs/heads/main", Old: id("b").String()}, got[5])
}

package repo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

func TestSetObserverReachesEveryLayer(t *testing.T) {
	r := newRepo(t)
	var rec events.Recorder
	r.SetObserver(&rec)

	id, _, err := r.ODB.Write(&objects.Blob{Data: []byte("x")})
	require.NoError(t, err)
	require.NoError(t, r.Refs.Update("refs/heads/main", id))
	require.NoError(t, r.Work.WriteFile("f", []byte("x"), 0o644))
	ix := index.New()
	ix.Add(index.Entry{Path: "f", Mode: objects.ModeFile, ID: id})
	require.NoError(t, r.SaveIndex(ix))
	r.Emit(events.Event{Kind: events.StateWritten, Subject: "MERGE_HEAD"})

	var kinds []events.Kind
	for _, e := range rec.Events() {
		kinds = append(kinds, e.Kind)
	}
	assert.Equal(t, []events.Kind{events.ObjectWritten, events.RefUpdated, events.FileWritten, events.IndexAdded, events.StateWritten}, kinds)

	r.SetObserver(nil)
	require.NoError(t, r.Work.WriteFile("g", []byte("y"), 0o644))
	r.Emit(events.Event{Kind: events.StateWritten})
	assert.Len(t, rec.Events(), 5, "nothing is reported after the observer is removed")
}

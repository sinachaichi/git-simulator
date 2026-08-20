package events_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/events"
)

func TestRecorderKeepsOrderAndReturnsACopy(t *testing.T) {
	var rec events.Recorder
	rec.Emit(events.Event{Kind: events.ObjectWritten, Subject: "a"})
	rec.Emit(events.Event{Kind: events.RefUpdated, Subject: "b"})

	got := rec.Events()
	assert.Equal(t, []events.Kind{events.ObjectWritten, events.RefUpdated}, []events.Kind{got[0].Kind, got[1].Kind})
	got[0].Subject = "changed"
	assert.Equal(t, "a", rec.Events()[0].Subject)

	rec.Reset()
	assert.Empty(t, rec.Events())
}

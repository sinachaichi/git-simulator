package odb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

func TestWriteReportsNewAndExistingObjects(t *testing.T) {
	s := newStore(t)
	var rec events.Recorder
	s.Observer = &rec

	_, _, err := s.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)
	_, _, err = s.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)

	got := rec.Events()
	require.Len(t, got, 2)
	assert.Equal(t, events.ObjectWritten, got[0].Kind)
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a", got[0].Subject)
	assert.Equal(t, "blob 6 bytes", got[0].Detail)
	assert.Equal(t, events.ObjectExists, got[1].Kind)
}

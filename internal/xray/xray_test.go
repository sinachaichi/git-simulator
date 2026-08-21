package xray_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/term"
	"github.com/sinachaichi/git-simulator/internal/xray"
)

const (
	blobID   = "ce013625030ba8dba906f756967f9e9ca394464a"
	commitID = "83eef8290a79d185273e8a8d55c46192d650c6f4"
	zero     = "0000000000000000000000000000000000000000"
)

func TestRenderGroupsEventsBySection(t *testing.T) {
	var out bytes.Buffer
	xray.Render(&out, term.Style{}, []events.Event{
		{Kind: events.ObjectWritten, Subject: blobID, Detail: "blob 6 bytes"},
		{Kind: events.ObjectExists, Subject: blobID, Detail: "blob 6 bytes"},
		{Kind: events.IndexAdded, Subject: "hello.txt", New: blobID, Detail: "100644"},
		{Kind: events.IndexModified, Subject: "a.txt", Old: blobID, New: commitID},
		{Kind: events.IndexRemoved, Subject: "old.txt"},
		{Kind: events.IndexConflict, Subject: "c.txt", Detail: "stage 2 (ours), stage 3 (theirs)"},
		{Kind: events.RefUpdated, Subject: "refs/heads/main", Old: zero, New: commitID},
		{Kind: events.HeadSet, New: "refs/heads/main", Detail: "symbolic"},
		{Kind: events.HeadSet, New: commitID, Detail: "detached"},
		{Kind: events.ReflogAppended, Subject: "HEAD", Detail: "commit (initial): first"},
		{Kind: events.RefDeleted, Subject: "refs/heads/old", Old: blobID},
		{Kind: events.FileWritten, Subject: "hello.txt", Detail: "6 bytes"},
		{Kind: events.FileRemoved, Subject: "gone.txt"},
		{Kind: events.StateWritten, Subject: "MERGE_HEAD", Detail: commitID},
		{Kind: events.StateRemoved, Subject: "MERGE_MSG"},
	})
	want := `x-ray
  objects
    + blob   ce01362  6 bytes
    = blob   ce01362  already stored, nothing written
  index
    + hello.txt  100644 ce01362
    ~ a.txt  ce01362 -> 83eef82
    - old.txt
    ! c.txt  stage 2 (ours), stage 3 (theirs)
  refs
    refs/heads/main  0000000 -> 83eef82
    HEAD -> refs/heads/main
    HEAD detached at 83eef82
    reflog HEAD: commit (initial): first
    refs/heads/old  deleted (was ce01362)
  working tree
    + wrote hello.txt  6 bytes
    - removed gone.txt
  state
    MERGE_HEAD written  83eef8290a79d185273e8a8d55c46192d650c6f4
    MERGE_MSG removed
`
	assert.Equal(t, want, out.String())
}

func TestRenderPrintsNothingWithoutEvents(t *testing.T) {
	var out bytes.Buffer
	xray.Render(&out, term.Style{}, nil)
	assert.Empty(t, out.String())
}

func TestRenderSkipsEmptySections(t *testing.T) {
	var out bytes.Buffer
	xray.Render(&out, term.Style{}, []events.Event{{Kind: events.FileRemoved, Subject: "a"}})
	assert.Equal(t, "x-ray\n  working tree\n    - removed a\n", out.String())
}

func TestRenderUsesColorsWhenEnabled(t *testing.T) {
	var out bytes.Buffer
	xray.Render(&out, term.Style{Enabled: true}, []events.Event{{Kind: events.FileRemoved, Subject: "a"}})
	assert.Contains(t, out.String(), "\x1b[")
}

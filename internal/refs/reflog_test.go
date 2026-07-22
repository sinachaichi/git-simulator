package refs_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
)

func who(unix int64) objects.Signature {
	return objects.Signature{
		Name: "Ada Lovelace", Email: "ada@example.com",
		When: time.Unix(unix, 0).In(time.FixedZone("", 7200)),
	}
}

func TestReflogNewestFirst(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{Old: objects.Zero, New: id("a"), Who: who(100), Message: "commit (initial): first"}))
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{Old: id("a"), New: id("b"), Who: who(200), Message: "commit: second"}))

	got, err := s.ReadLog("HEAD")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "commit: second", got[0].Message)
	assert.Equal(t, id("a"), got[0].Old)
	assert.Equal(t, id("b"), got[0].New)
	assert.Equal(t, int64(200), got[0].Who.When.Unix())
	assert.True(t, got[1].Old.IsZero())
}

func TestReflogMissingIsEmpty(t *testing.T) {
	got, err := newStore(t).ReadLog("refs/heads/none")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReflogKeepsMessageOnOneLine(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{New: id("a"), Who: who(1), Message: "line one\nline two"}))
	got, err := s.ReadLog("HEAD")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "line one line two", got[0].Message)
}

func TestReflogNestedRefName(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.AppendLog("refs/heads/feat/x", refs.LogEntry{New: id("a"), Who: who(1), Message: "branch: Created"}))
	got, err := s.ReadLog("refs/heads/feat/x")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestReflogCorruptLine(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{New: id("a"), Who: who(1), Message: "ok"}))
	require.NoError(t, s.AppendLog("HEAD", refs.LogEntry{New: id("a"), Who: who(1), Message: "ok"}))
	// A line that does not follow the format.
	p := s.Dir + "/logs/HEAD"
	require.NoError(t, appendFile(p, "garbage line\n"))
	_, err := s.ReadLog("HEAD")
	assert.ErrorContains(t, err, "reflog of HEAD")
}

func appendFile(p, text string) error {
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	return errors.Join(err, f.Close())
}

func TestMoveAndDeleteLog(t *testing.T) {
	s := newStore(t)
	require.NoError(t, s.AppendLog("refs/heads/old", refs.LogEntry{New: id("a"), Who: who(1), Message: "branch: Created"}))
	require.NoError(t, s.MoveLog("refs/heads/old", "refs/heads/feat/new"))

	got, err := s.ReadLog("refs/heads/feat/new")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	got, err = s.ReadLog("refs/heads/old")
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, s.DeleteLog("refs/heads/feat/new"))
	assert.NoDirExists(t, s.Dir+"/logs/refs/heads/feat")
	assert.NoError(t, s.DeleteLog("refs/heads/feat/new"), "deleting a missing log is fine")
	assert.NoError(t, s.MoveLog("refs/heads/none", "refs/heads/x"))
}

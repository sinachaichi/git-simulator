package porcelain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func TestReflogRecordsEveryMoveOfHead(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "first")
	_, err := porcelain.Switch(r, "topic", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	write(t, r, "a", "2\n")
	commitAll(t, r, "second")
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	_, err = porcelain.Reset(r, "topic", porcelain.ResetHard)
	require.NoError(t, err)

	lines, err := porcelain.Reflog(r, "")
	require.NoError(t, err)
	var got []string
	for _, l := range lines {
		got = append(got, l.Selector+" "+l.Message)
	}
	assert.Equal(t, []string{
		"HEAD@{0} reset: moving to topic",
		"HEAD@{1} checkout: moving from topic to main",
		"HEAD@{2} commit: second",
		"HEAD@{3} checkout: moving from main to topic",
		"HEAD@{4} commit (initial): first",
	}, got)
}

func TestReflogOfABranchAndOfStash(t *testing.T) {
	r := twoCommits(t)
	lines, err := porcelain.Reflog(r, "main")
	require.NoError(t, err)
	require.Len(t, lines, 2)
	assert.Equal(t, "main@{0}", lines[0].Selector)
	assert.Equal(t, "commit: second", lines[0].Message)

	lines, err = porcelain.Reflog(r, "stash")
	require.NoError(t, err)
	assert.Empty(t, lines)

	_, err = porcelain.Reflog(r, "ghost")
	assert.True(t, errors.Is(err, porcelain.ErrBranchNotFound))
}

// The reflog is what makes a lost commit recoverable.
func TestReflogLetsYouRecoverAfterAHardReset(t *testing.T) {
	r := threeCommits(t)
	lost := headID(t, r)
	_, err := porcelain.Reset(r, "HEAD~2", porcelain.ResetHard)
	require.NoError(t, err)

	lines, err := porcelain.Reflog(r, "")
	require.NoError(t, err)
	assert.Equal(t, lost, lines[1].ID, "HEAD@{1} is where we were before the reset")

	_, err = porcelain.Reset(r, "HEAD@{1}", porcelain.ResetHard)
	require.NoError(t, err)
	assert.Equal(t, lost, headID(t, r))
	assert.Equal(t, "v3\n", read(t, r, "a.txt"))
}

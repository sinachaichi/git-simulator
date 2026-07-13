package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagsAllowsFlagsAfterArguments(t *testing.T) {
	fs := newFlags("t")
	all := fs.Bool("a", false, "")
	msg := fs.String("m", "", "")
	rest, err := parseFlags(fs, []string{"file1", "-a", "file2", "-m", "hello world"})
	require.NoError(t, err)
	assert.Equal(t, []string{"file1", "file2"}, rest)
	assert.True(t, *all)
	assert.Equal(t, "hello world", *msg)
}

func TestParseFlagsGroupedShortFlags(t *testing.T) {
	fs := newFlags("t")
	all := fs.Bool("a", false, "")
	msg := fs.String("m", "", "")
	rest, err := parseFlags(fs, []string{"-am", "text"})
	require.NoError(t, err)
	assert.Empty(t, rest)
	assert.True(t, *all)
	assert.Equal(t, "text", *msg)
}

func TestParseFlagsRepeatedValue(t *testing.T) {
	fs := newFlags("t")
	var msgs stringList
	fs.Var(&msgs, "m", "")
	_, err := parseFlags(fs, []string{"-m", "one", "-m", "two"})
	require.NoError(t, err)
	assert.Equal(t, stringList{"one", "two"}, msgs)
}

func TestParseFlagsEqualsAndDoubleDash(t *testing.T) {
	fs := newFlags("t")
	n := fs.Int("n", 0, "")
	rest, err := parseFlags(fs, []string{"--n=3", "--", "-weird-name"})
	require.NoError(t, err)
	assert.Equal(t, 3, *n)
	assert.Equal(t, []string{"-weird-name"}, rest)
}

func TestParseFlagsLongFlagIsNotSplit(t *testing.T) {
	fs := newFlags("t")
	oneline := fs.Bool("oneline", false, "")
	_, err := parseFlags(fs, []string{"-oneline"})
	require.NoError(t, err)
	assert.True(t, *oneline)
}

func TestParseFlagsUnknownFlagIsAUsageError(t *testing.T) {
	_, err := parseFlags(newFlags("t"), []string{"--nope"})
	var u *usageError
	require.ErrorAs(t, err, &u)
	assert.Contains(t, err.Error(), "-nope")
}

func TestParseFlagsKeepsSingleDashAsArgument(t *testing.T) {
	rest, err := parseFlags(newFlags("t"), []string{"-"})
	require.NoError(t, err)
	assert.Equal(t, []string{"-"}, rest)
}

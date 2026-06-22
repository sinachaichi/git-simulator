package index_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

func entry(path, content string) index.Entry {
	return index.Entry{Path: path, Mode: objects.ModeFile, ID: blob(content), Size: uint32(len(content))}
}

func TestAddKeepsEntriesSorted(t *testing.T) {
	ix := index.New()
	for _, p := range []string{"b", "a/z", "a/b", "c"} {
		ix.Add(entry(p, p))
	}
	assert.Equal(t, []string{"a/b", "a/z", "b", "c"}, ix.Paths())
}

func TestAddReplacesExistingEntry(t *testing.T) {
	ix := index.New()
	ix.Add(entry("a", "old"))
	ix.Add(entry("a", "new"))
	assert.Equal(t, 1, ix.Len())
	got, ok := ix.Get("a")
	require.True(t, ok)
	assert.Equal(t, blob("new"), got.ID)
}

func TestAddFileReplacesDirectory(t *testing.T) {
	ix := index.New()
	ix.Add(entry("a/b", "1"))
	ix.Add(entry("a/c", "2"))
	ix.Add(entry("ab", "3"))
	ix.Add(entry("a", "4"))
	assert.Equal(t, []string{"a", "ab"}, ix.Paths())
}

func TestAddNestedReplacesFile(t *testing.T) {
	ix := index.New()
	ix.Add(entry("a", "1"))
	ix.Add(entry("a/b", "2"))
	assert.Equal(t, []string{"a/b"}, ix.Paths())
}

func TestRemove(t *testing.T) {
	ix := index.New()
	ix.Add(entry("a", "1"))
	assert.True(t, ix.Remove("a"))
	assert.False(t, ix.Remove("a"))
	_, ok := ix.Get("a")
	assert.False(t, ok)
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	ix := index.New()
	ix.Add(entry("a/b", "1"))
	ix.Add(entry("c", "2"))
	require.NoError(t, ix.Save(path))

	got, err := index.Load(path)
	require.NoError(t, err)
	assert.Equal(t, ix.Entries(), got.Entries())
}

func TestLoadMissingFileGivesEmptyIndex(t *testing.T) {
	ix, err := index.Load(filepath.Join(t.TempDir(), "index"))
	require.NoError(t, err)
	assert.Zero(t, ix.Len())
}

func TestEntriesReturnsACopy(t *testing.T) {
	ix := index.New()
	ix.Add(entry("a", "1"))
	ix.Entries()[0].Path = "changed"
	assert.Equal(t, []string{"a"}, ix.Paths())
}

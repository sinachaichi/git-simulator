package objects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

func blobID(s string) objects.ID {
	return objects.HashOf(&objects.Blob{Data: []byte(s)})
}

// Expected id from real git: one file "hello.txt" with content "hello\n".
func TestTreeIDMatchesGit(t *testing.T) {
	tree := &objects.Tree{Entries: []objects.TreeEntry{
		{Mode: objects.ModeFile, Name: "hello.txt", ID: blobID("hello\n")},
	}}
	assert.Equal(t, "aaa96ced2d9a1c8e72c56b253a0e2fe78393feb7", objects.HashOf(tree).String())
}

func TestEmptyTreeID(t *testing.T) {
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", objects.HashOf(&objects.Tree{}).String())
}

func TestTreeSortPutsDirsAsIfTheyEndWithSlash(t *testing.T) {
	tree := &objects.Tree{Entries: []objects.TreeEntry{
		{Mode: objects.ModeFile, Name: "a.txt"},
		{Mode: objects.ModeDir, Name: "a"},
		{Mode: objects.ModeFile, Name: "a-b"},
	}}
	tree.Sort()
	var names []string
	for _, e := range tree.Entries {
		names = append(names, e.Name)
	}
	assert.Equal(t, []string{"a-b", "a.txt", "a"}, names)
}

func TestTreeRoundTrip(t *testing.T) {
	tree := &objects.Tree{Entries: []objects.TreeEntry{
		{Mode: objects.ModeExecutable, Name: "run.sh", ID: blobID("#!/bin/sh\n")},
		{Mode: objects.ModeDir, Name: "src", ID: blobID("x")},
	}}
	got, err := objects.ParseTree(tree.Body())
	require.NoError(t, err)
	assert.Equal(t, tree, got)
	assert.True(t, got.Entries[1].IsDir())
}

func TestParseTreeRejectsGarbage(t *testing.T) {
	for _, body := range []string{"100644", "100644 name", "zzz name\x00" + string(make([]byte, 20)), "100644 a/b\x00" + string(make([]byte, 20))} {
		_, err := objects.ParseTree([]byte(body))
		assert.Error(t, err, body)
	}
}

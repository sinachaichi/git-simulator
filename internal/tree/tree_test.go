package tree_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

func newStore(t *testing.T) *odb.Store {
	t.Helper()
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	return s
}

func file(t *testing.T, s *odb.Store, path, content string) tree.File {
	t.Helper()
	id, _, err := s.Write(&objects.Blob{Data: []byte(content)})
	require.NoError(t, err)
	return tree.File{Path: path, Mode: objects.ModeFile, ID: id}
}

func TestBuildThenFlatten(t *testing.T) {
	s := newStore(t)
	files := []tree.File{
		file(t, s, "src/workdir/file1.go", "1"),
		file(t, s, "README.md", "readme"),
		file(t, s, "src/main.go", "main"),
		file(t, s, "src-extra.txt", "x"),
	}
	root, err := tree.Build(s, files)
	require.NoError(t, err)

	got, err := tree.Flatten(s, root)
	require.NoError(t, err)
	var paths []string
	for _, f := range got {
		paths = append(paths, f.Path)
	}
	// Git order: "src-extra.txt" sorts before the "src/" directory.
	assert.Equal(t, []string{"README.md", "src-extra.txt", "src/main.go", "src/workdir/file1.go"}, paths)
}

func TestBuildIsDeterministic(t *testing.T) {
	s := newStore(t)
	a := file(t, s, "a/x", "1")
	b := file(t, s, "b", "2")
	id1, err := tree.Build(s, []tree.File{a, b})
	require.NoError(t, err)
	id2, err := tree.Build(s, []tree.File{b, a})
	require.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func TestBuildEmpty(t *testing.T) {
	id, err := tree.Build(newStore(t), nil)
	require.NoError(t, err)
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", id.String())
}

func TestBuildRejectsFileDirectoryClash(t *testing.T) {
	s := newStore(t)
	_, err := tree.Build(s, []tree.File{file(t, s, "a", "1"), file(t, s, "a/b", "2")})
	assert.ErrorContains(t, err, "both a file and a directory")
}

func TestFlattenKeepsModes(t *testing.T) {
	s := newStore(t)
	f := file(t, s, "run.sh", "#!/bin/sh")
	f.Mode = objects.ModeExecutable
	root, err := tree.Build(s, []tree.File{f})
	require.NoError(t, err)
	got, err := tree.Flatten(s, root)
	require.NoError(t, err)
	assert.Equal(t, []tree.File{f}, got)
}

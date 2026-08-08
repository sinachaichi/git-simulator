package merge_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

type fixture struct {
	t *testing.T
	s *odb.Store
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	return &fixture{t: t, s: s}
}

func (f *fixture) file(path, content string) tree.File {
	f.t.Helper()
	id, _, err := f.s.Write(&objects.Blob{Data: []byte(content)})
	require.NoError(f.t, err)
	return tree.File{Path: path, Mode: objects.ModeFile, ID: id}
}

func (f *fixture) content(id objects.ID) string {
	f.t.Helper()
	b, err := f.s.ReadBlob(id)
	require.NoError(f.t, err)
	return string(b.Data)
}

func (f *fixture) merge(base, ours, theirs []tree.File) merge.Result {
	f.t.Helper()
	res, err := merge.Trees(f.s, base, ours, theirs, labels)
	require.NoError(f.t, err)
	return res
}

func paths(files []tree.File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

func TestTreesTakeEachSidesChanges(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("keep", "k"), f.file("edit-ours", "1"), f.file("edit-theirs", "1"), f.file("del-ours", "d"), f.file("del-theirs", "d")}
	ours := []tree.File{f.file("keep", "k"), f.file("edit-ours", "2"), f.file("edit-theirs", "1"), f.file("del-theirs", "d"), f.file("add-ours", "a")}
	theirs := []tree.File{f.file("keep", "k"), f.file("edit-ours", "1"), f.file("edit-theirs", "2"), f.file("del-ours", "d"), f.file("add-theirs", "b")}

	res := f.merge(base, ours, theirs)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, []string{"add-ours", "add-theirs", "edit-ours", "edit-theirs", "keep"}, paths(res.Files))
	byName := map[string]string{}
	for _, file := range res.Files {
		byName[file.Path] = f.content(file.ID)
	}
	assert.Equal(t, "2", byName["edit-ours"])
	assert.Equal(t, "2", byName["edit-theirs"])
}

func TestTreesBothSidesMakeTheSameChange(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("a", "1"), f.file("gone", "x")}
	same := []tree.File{f.file("a", "2"), f.file("new", "n")}
	res := f.merge(base, same, same)
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, []string{"a", "new"}, paths(res.Files))
}

func TestTreesMergeTextInTheSameFile(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("f", lines(10))}
	ours := []tree.File{f.file("f", "top\n"+lines(10))}
	theirs := []tree.File{f.file("f", lines(10)+"bottom\n")}
	res := f.merge(base, ours, theirs)
	require.Empty(t, res.Conflicts)
	assert.Equal(t, "top\n"+lines(10)+"bottom\n", f.content(res.Files[0].ID))
}

func TestTreesContentConflictKeepsMarkers(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("f", "a\nb\nc\n")}
	ours := []tree.File{f.file("f", "a\nours\nc\n")}
	theirs := []tree.File{f.file("f", "a\ntheirs\nc\n")}
	res := f.merge(base, ours, theirs)
	assert.Empty(t, res.Files)
	require.Len(t, res.Conflicts, 1)
	c := res.Conflicts[0]
	assert.Equal(t, merge.ContentConflict, c.Kind)
	assert.Equal(t, "a\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\nc\n", string(c.Content))
	assert.NotNil(t, c.Base)
	assert.NotNil(t, c.Ours)
	assert.NotNil(t, c.Theirs)
}

func TestTreesAddAddConflict(t *testing.T) {
	f := newFixture(t)
	res := f.merge(nil, []tree.File{f.file("n", "ours\n")}, []tree.File{f.file("n", "theirs\n")})
	require.Len(t, res.Conflicts, 1)
	c := res.Conflicts[0]
	assert.Equal(t, merge.AddAdd, c.Kind)
	assert.Nil(t, c.Base)
	assert.Equal(t, "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n", string(c.Content))
}

func TestTreesModifyDeleteConflict(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("f", "1\n")}
	edited := []tree.File{f.file("f", "2\n")}

	res := f.merge(base, nil, edited)
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, merge.ModifyDelete, res.Conflicts[0].Kind)
	assert.Nil(t, res.Conflicts[0].Ours)
	assert.Equal(t, "2\n", string(res.Conflicts[0].Content))

	res = f.merge(base, edited, nil)
	require.Len(t, res.Conflicts, 1)
	assert.Nil(t, res.Conflicts[0].Theirs)
	assert.Equal(t, "2\n", string(res.Conflicts[0].Content))
}

func TestTreesBinaryConflict(t *testing.T) {
	f := newFixture(t)
	base := []tree.File{f.file("img", "\x00a")}
	res := f.merge(base, []tree.File{f.file("img", "\x00b")}, []tree.File{f.file("img", "\x00c")})
	require.Len(t, res.Conflicts, 1)
	assert.Equal(t, merge.BinaryConflict, res.Conflicts[0].Kind)
	assert.Equal(t, "\x00b", string(res.Conflicts[0].Content), "our version stays in the working tree")
}

func TestTreesModeChanges(t *testing.T) {
	f := newFixture(t)
	plain := f.file("run.sh", "#!/bin/sh\n")
	exec := plain
	exec.Mode = objects.ModeExecutable

	res := f.merge([]tree.File{plain}, []tree.File{exec}, []tree.File{plain})
	assert.Empty(t, res.Conflicts)
	assert.Equal(t, objects.ModeExecutable, res.Files[0].Mode)

	// One side edits, the other makes it executable: both survive.
	edited := f.file("run.sh", "#!/bin/sh\necho hi\n")
	res = f.merge([]tree.File{plain}, []tree.File{exec}, []tree.File{edited})
	require.Empty(t, res.Conflicts)
	assert.Equal(t, objects.ModeExecutable, res.Files[0].Mode)
	assert.Equal(t, "#!/bin/sh\necho hi\n", f.content(res.Files[0].ID))
}

func TestTreesRejectsFileDirectoryClash(t *testing.T) {
	f := newFixture(t)
	_, err := merge.Trees(f.s, nil, []tree.File{f.file("a", "x")}, []tree.File{f.file("a/b", "y")}, labels)
	assert.ErrorContains(t, err, "file on one side and a directory on the other")
}

func TestTreesEmptyInputs(t *testing.T) {
	f := newFixture(t)
	res := f.merge(nil, nil, nil)
	assert.Empty(t, res.Files)
	assert.Empty(t, res.Conflicts)
}

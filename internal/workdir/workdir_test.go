package workdir_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/testutil"
	"github.com/sinachaichi/git-simulator/internal/workdir"
)

func newTree(t *testing.T) *workdir.WorkDir {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{
		"README.md":            "### MY GIT IMPL",
		"src/main.go":          "",
		"src/workdir/file1.go": "",
		"src/workdir/file2.go": "",
	})
	wd, err := workdir.New(root)
	require.NoError(t, err)
	return wd
}

func TestList(t *testing.T) {
	wd := newTree(t)
	files, err := wd.List()
	require.NoError(t, err)
	assert.Equal(t, []string{
		"README.md",
		"src/main.go",
		"src/workdir/file1.go",
		"src/workdir/file2.go",
	}, files)
}

func TestListIn(t *testing.T) {
	wd := newTree(t)
	files, err := wd.ListIn("src")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"src/main.go",
		"src/workdir/file1.go",
		"src/workdir/file2.go",
	}, files)
}

func TestListIgnoresMetaDir(t *testing.T) {
	wd := newTree(t)
	require.NoError(t, wd.WriteFile("a.txt", []byte("a"), 0o644))
	testutil.WriteFiles(t, wd.Root, map[string]string{".hood/HEAD": "ref: refs/heads/main\n"})
	files, err := wd.List()
	require.NoError(t, err)
	assert.NotContains(t, files, ".hood/HEAD")
}

func TestReadFile(t *testing.T) {
	wd := newTree(t)
	data, err := wd.ReadFile("README.md")
	require.NoError(t, err)
	assert.Equal(t, "### MY GIT IMPL", string(data))
}

func TestReadFileNotFound(t *testing.T) {
	_, err := newTree(t).ReadFile("src/non_existing_file")
	assert.Error(t, err)
}

func TestAppendFile(t *testing.T) {
	wd := newTree(t)
	require.NoError(t, wd.AppendFile("src/main.go", []byte("package main")))
	require.NoError(t, wd.AppendFile("src/main.go", []byte("\nfunc main(){}\n")))
	data, err := wd.ReadFile("src/main.go")
	require.NoError(t, err)
	assert.Equal(t, "package main\nfunc main(){}\n", string(data))
}

func TestAppendFileNotFound(t *testing.T) {
	assert.Error(t, newTree(t).AppendFile("src/non_existing_file", []byte("salam")))
}

func TestWriteFileCreatesParents(t *testing.T) {
	wd := newTree(t)
	require.NoError(t, wd.WriteFile("deep/er/file.txt", []byte("x"), 0o644))
	assert.True(t, wd.Exists("deep/er/file.txt"))
}

func TestRemovePrunesEmptyDirs(t *testing.T) {
	wd := newTree(t)
	require.NoError(t, wd.Remove("src/workdir/file1.go"))
	require.NoError(t, wd.Remove("src/workdir/file2.go"))
	_, err := wd.Stat("src/workdir")
	assert.Error(t, err)
	assert.True(t, wd.Exists("src/main.go"))
}

func TestRejectsEscapingPaths(t *testing.T) {
	wd := newTree(t)
	for _, p := range []string{"../x", "/etc/passwd", ".", ".hood/HEAD", "a/../../x"} {
		_, err := wd.ReadFile(p)
		assert.Error(t, err, p)
	}
}

func TestNewRejectsFile(t *testing.T) {
	wd := newTree(t)
	_, err := workdir.New(wd.Root + "/README.md")
	assert.Error(t, err)
}

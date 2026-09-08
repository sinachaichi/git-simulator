package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binary string

// TestMain builds the real binary once. These tests run it the way a
// user does, as a separate process with its own streams and exit code.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hood-e2e")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "hood")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=e2e", "-o", binary, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func hood(t *testing.T, dir, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOOD_HOME="+filepath.Join(dir, ".home"), "NO_COLOR=1", "HOOD_XRAY=",
		"GIT_AUTHOR_NAME=Ada Lovelace", "GIT_AUTHOR_EMAIL=ada@example.com",
		"GIT_COMMITTER_NAME=Ada Lovelace", "GIT_COMMITTER_EMAIL=ada@example.com",
		"GIT_AUTHOR_DATE=1700000000 +0000", "GIT_COMMITTER_DATE=1700000000 +0000")
	cmd.Stdin = strings.NewReader(stdin)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	code = 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errBuf.String(), code
}

func TestVersionAndHelp(t *testing.T) {
	out, _, code := hood(t, t.TempDir(), "", "--version")
	assert.Equal(t, 0, code)
	assert.Equal(t, "hood e2e\n", out)

	out, _, code = hood(t, t.TempDir(), "", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "cherry-pick")
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	_, _, code := hood(t, dir, "", "nonsense")
	assert.Equal(t, 2, code, "unknown command")
	_, _, code = hood(t, dir, "", "status")
	assert.Equal(t, 1, code, "no repository")
	_, _, code = hood(t, dir, "", "init")
	assert.Equal(t, 0, code)
	_, _, code = hood(t, dir, "", "add")
	assert.Equal(t, 2, code, "usage error")
}

// The whole story in one go: commit, branch, diverge, conflict, resolve,
// then undo and recover, always through the real binary.
func TestFullWorkflow(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, errOut, code := hood(t, dir, "", args...)
		require.Equal(t, 0, code, "hood %s\n%s", strings.Join(args, " "), errOut)
		return out
	}
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}

	run("init")
	write("story.txt", "intro\nmiddle\nend\n")
	run("add", ".")
	run("commit", "-m", "story")
	run("switch", "-c", "feature")
	write("story.txt", "intro\nfeature middle\nend\n")
	run("commit", "-am", "feature edit")
	run("switch", "main")
	write("story.txt", "intro\nmain middle\nend\n")
	run("commit", "-am", "main edit")

	out, _, code := hood(t, dir, "", "merge", "feature")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "CONFLICT (content): Merge conflict in story.txt")
	write("story.txt", "intro\nboth middle\nend\n")
	run("add", "story.txt")
	run("commit")
	assert.Contains(t, run("log", "--oneline", "-n", "1"), "Merge branch 'feature'")
	assert.Contains(t, run("log", "--graph", "--oneline"), "|\\")

	// A hard reset loses the merge, and the reflog gets it back.
	run("reset", "--hard", "HEAD~1")
	assert.NotContains(t, run("log", "--oneline"), "Merge branch")
	run("reset", "--hard", "HEAD@{1}")
	assert.Contains(t, run("log", "--oneline"), "Merge branch 'feature'")
	assert.Equal(t, "On branch main\nnothing to commit, working tree clean\n", run("status"))
}

func TestShellThroughAPipe(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := hood(t, dir, "init\nwrite a.txt hi\nadd a.txt\nxray off\nstatus\nexit\n", "shell")
	assert.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "hood e2e. Type `help` for commands")
	assert.Contains(t, out, "+ blob   ")
	assert.Contains(t, out, "new file:   a.txt")
}

func TestPlainHoodWithoutATerminalOpensTheShellToo(t *testing.T) {
	out, _, code := hood(t, t.TempDir(), "exit\n")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "hood e2e.")
}

func TestRefusesToWorkInsideARealGitRepository(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	_, errOut, code := hood(t, dir, "", "init")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "real git repository")
	assert.NoDirExists(t, filepath.Join(dir, ".hood"))
}

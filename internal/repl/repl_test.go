package repl_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/repl"
)

// shell runs a script of lines through the shell in dir and returns what
// it printed on stdout and stderr, plus the exit code.
func shell(t *testing.T, dir string, lines ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Ada Lovelace")
	t.Setenv("GIT_AUTHOR_EMAIL", "ada@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ada Lovelace")
	t.Setenv("GIT_COMMITTER_EMAIL", "ada@example.com")
	var out, errBuf bytes.Buffer
	env := cli.Env{Stdin: strings.NewReader(strings.Join(lines, "\n") + "\n"), Stdout: &out, Stderr: &errBuf, Cwd: dir}
	code = repl.Run(env, "test")
	return out.String(), errBuf.String(), code
}

func TestShellRunsCommandsWithXrayOn(t *testing.T) {
	dir := t.TempDir()
	out, errOut, code := shell(t, dir,
		"init",
		"write hello.txt hello world",
		"add hello.txt",
		`commit -m "first commit"`,
		"exit")
	assert.Equal(t, 0, code)
	assert.Empty(t, errOut)
	assert.Contains(t, out, "hood test. Type `help` for commands")
	assert.Contains(t, out, "hood> init\n")
	assert.Contains(t, out, "hood (main)> add hello.txt\n")
	assert.Contains(t, out, "    + blob   ")
	assert.Contains(t, out, "[main (root-commit) ")
	assert.Contains(t, out, "    reflog HEAD: commit (initial): first commit\n")
	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", string(data))
}

func TestShellKeepsGoingAfterErrors(t *testing.T) {
	out, errOut, code := shell(t, t.TempDir(),
		"status",
		"frobnicate",
		`commit -m "unclosed`,
		"cat nothing.txt",
		"init",
		"status")
	assert.Equal(t, 0, code, "EOF ends the shell cleanly")
	assert.Contains(t, errOut, "not a hood repository")
	assert.Contains(t, errOut, `"frobnicate" is not a command`)
	assert.Contains(t, errOut, "unclosed quote")
	assert.Contains(t, errOut, "nothing.txt")
	assert.Contains(t, out, "On branch main")
}

func TestShellAcceptsGitAndHoodPrefixes(t *testing.T) {
	out, _, _ := shell(t, t.TempDir(), "git init", "hood status", "git")
	assert.Contains(t, out, "Initialized empty hood repository")
	assert.Contains(t, out, "On branch main")
}

func TestXrayAndLiveToggles(t *testing.T) {
	dir := t.TempDir()
	out, _, _ := shell(t, dir,
		"xray", "xray off", "init", "write a hi", "add a", "xray on", "write b yo", "add b",
		"commit -m one", "live on", "write c z", "add c", "commit -m two", "live off", "xray maybe")
	assert.Contains(t, out, "x-ray is on\n")
	assert.Contains(t, out, "x-ray is off\n")
	// X-ray was off for init and the first add, and on for the other four commands.
	assert.Equal(t, 4, strings.Count(out, "\nx-ray\n"))
	assert.Contains(t, out, "live graph is on\n")
	assert.Contains(t, out, "* ", "the live graph is drawn")
	assert.Contains(t, out, "live graph is off\n")
}

func TestShellFileHelpers(t *testing.T) {
	dir := t.TempDir()
	out, errOut, _ := shell(t, dir,
		"write notes.txt first line", "append notes.txt second line", "cat notes.txt",
		"write dir/deep.txt x", "ls", "del notes.txt", "ls dir", "pwd",
		"write ../escape.txt no", "cat ../escape.txt", "write .hood/HEAD x", "del missing.txt")
	assert.Contains(t, out, "first line\nsecond line\n")
	assert.Contains(t, out, "dir/deep.txt\nnotes.txt\n")
	assert.Contains(t, out, dir)
	assert.Contains(t, errOut, "outside the working tree")
	assert.Contains(t, errOut, "inside .hood")
	assert.NoFileExists(t, filepath.Join(filepath.Dir(dir), "escape.txt"))
}

func TestCdChangesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "project"), 0o755))
	out, errOut, _ := shell(t, dir, "cd project", "init", "pwd", "cd ..", "pwd", "cd nowhere")
	assert.Contains(t, out, filepath.Join(dir, "project")+"\n")
	assert.DirExists(t, filepath.Join(dir, "project", ".hood"))
	assert.Contains(t, errOut, "nowhere is not a directory")
}

func TestHelp(t *testing.T) {
	out, errOut, _ := shell(t, t.TempDir(), "help", "help commit", "help xray", "help nothing")
	assert.Contains(t, out, "hood commands:")
	assert.Contains(t, out, "shell helpers:")
	assert.Contains(t, out, "commit -m <message> [-a] [--amend] [--allow-empty]\n  record the staged changes")
	assert.Contains(t, out, "xray [on|off]\n  show or change X-ray mode")
	assert.Contains(t, errOut, `no help for "nothing"`)
}

func TestLessonStartMovesTheShellIntoTheWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOOD_HOME", home)
	out, errOut, _ := shell(t, t.TempDir(), "lesson start first-commit", "status", "add README.md", "lesson check", "xray off", "lesson check")
	assert.Empty(t, errOut)
	assert.Contains(t, out, "You are now in the lesson repository: "+filepath.Join(home, "lessons", "first-commit"))
	assert.Contains(t, out, "hood (main)> status\nOn branch main\n\nNo commits yet")
	assert.Contains(t, out, "ok Step 1 done.")
}

func TestPromptShowsDetachedHead(t *testing.T) {
	out, _, _ := shell(t, t.TempDir(), "init", "write a x", "add a", "commit -m one", "checkout --detach main", "status")
	assert.Regexp(t, `hood \([0-9a-f]{7}\)> status`, out)
}

func TestEmptyLinesAreIgnoredAndExitAliases(t *testing.T) {
	out, _, code := shell(t, t.TempDir(), "", "   ", "quit", "init")
	assert.Equal(t, 0, code)
	assert.NotContains(t, out, "Initialized", "nothing runs after quit")
}

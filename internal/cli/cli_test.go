package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

// session runs commands the way a user would, in one directory.
type session struct {
	t   *testing.T
	dir string
}

func newSession(t *testing.T) *session {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Ada Lovelace")
	t.Setenv("GIT_AUTHOR_EMAIL", "ada@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ada Lovelace")
	t.Setenv("GIT_COMMITTER_EMAIL", "ada@example.com")
	t.Setenv("GIT_AUTHOR_DATE", "1700000000 +0100")
	t.Setenv("GIT_COMMITTER_DATE", "1700000000 +0100")
	t.Setenv("NO_COLOR", "1")
	return &session{t: t, dir: t.TempDir()}
}

func (s *session) run(args ...string) (stdout, stderr string, code int) {
	s.t.Helper()
	var out, errBuf bytes.Buffer
	env := cli.Env{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errBuf, Cwd: s.dir}
	code = cli.Run(args, env, "test")
	return out.String(), errBuf.String(), code
}

func (s *session) ok(args ...string) string {
	s.t.Helper()
	out, errOut, code := s.run(args...)
	require.Equal(s.t, 0, code, "hood %s\nstderr: %s", strings.Join(args, " "), errOut)
	return out
}

func (s *session) write(name, content string) {
	s.t.Helper()
	p := filepath.Join(s.dir, name)
	require.NoError(s.t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(s.t, os.WriteFile(p, []byte(content), 0o644))
}

func TestVersionAndHelp(t *testing.T) {
	s := newSession(t)
	assert.Equal(t, "hood test\n", s.ok("--version"))
	out := s.ok("--help")
	assert.Contains(t, out, "usage: hood")
	assert.Contains(t, out, "commit")
	assert.Equal(t, out, s.ok())
}

func TestUnknownCommandAndOption(t *testing.T) {
	s := newSession(t)
	_, errOut, code := s.run("frobnicate")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, `"frobnicate" is not a command`)
	_, errOut, code = s.run("--bogus")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "unknown option --bogus")
}

func TestCommandsOutsideARepo(t *testing.T) {
	s := newSession(t)
	_, errOut, code := s.run("status")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "not inside a hood repository")
}

func TestFirstCommitSession(t *testing.T) {
	s := newSession(t)
	out := s.ok("init")
	assert.Contains(t, out, "Initialized empty hood repository in")

	assert.Equal(t, "On branch main\n\nNo commits yet\n\nnothing to commit (create/copy files and use \"add\" to track)\n", s.ok("status"))

	s.write("hello.txt", "hello\n")
	out = s.ok("status")
	assert.Contains(t, out, "Untracked files:")
	assert.Contains(t, out, "\thello.txt")
	assert.Contains(t, out, "nothing added to commit but untracked files present")

	s.ok("add", "hello.txt")
	out = s.ok("status")
	assert.Contains(t, out, "Changes to be committed:")
	assert.Contains(t, out, "\tnew file:   hello.txt")

	out = s.ok("commit", "-m", "first commit")
	assert.Regexp(t, `^\[main \(root-commit\) [0-9a-f]{7}\] first commit\n$`, out)
	assert.Equal(t, "On branch main\nnothing to commit, working tree clean\n", s.ok("status"))
}

func TestLogOutput(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.write("a", "2")
	s.ok("commit", "-am", "two")

	out := s.ok("log")
	assert.Contains(t, out, "(HEAD -> main)")
	assert.Contains(t, out, "Author: Ada Lovelace <ada@example.com>")
	assert.Contains(t, out, "Date:   Tue Nov 14 23:13:20 2023 +0100")
	assert.Contains(t, out, "    two\n")
	assert.Less(t, strings.Index(out, "two"), strings.Index(out, "one"))

	lines := strings.Split(strings.TrimSpace(s.ok("log", "--oneline")), "\n")
	require.Len(t, lines, 2)
	assert.Regexp(t, `^[0-9a-f]{7} \(HEAD -> main\) two$`, lines[0])
	assert.Regexp(t, `^[0-9a-f]{7} one$`, lines[1])

	assert.Len(t, strings.Split(strings.TrimSpace(s.ok("log", "--oneline", "-n", "1")), "\n"), 1)
}

func TestCommitErrors(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	_, errOut, code := s.run("commit", "-m", "x")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "nothing to commit")
	s.write("a", "1")
	s.ok("add", "a")
	_, errOut, code = s.run("commit")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "empty commit message")
}

func TestAddErrors(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	_, errOut, code := s.run("add")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "usage: add")
	_, errOut, code = s.run("add", "nope.txt")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "pathspec 'nope.txt' did not match any files")
	_, errOut, code = s.run("add", "../outside")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "outside the repository")
}

func TestSubdirectoryPaths(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("src/main.go", "package main\n")
	s.dir = filepath.Join(s.dir, "src")
	s.ok("add", "main.go")
	out := s.ok("ls-files")
	assert.Equal(t, "src/main.go\n", out)
}

func TestCatFileAndHashObject(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("hello.txt", "hello\n")
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a\n", s.ok("hash-object", "hello.txt"))
	_, _, code := s.run("cat-file", "-t", "ce01362")
	assert.Equal(t, 1, code, "not stored yet")
	s.ok("hash-object", "-w", "hello.txt")
	assert.Equal(t, "blob\n", s.ok("cat-file", "-t", "ce01362"))
	assert.Equal(t, "6\n", s.ok("cat-file", "-s", "ce01362"))
	assert.Equal(t, "hello\n", s.ok("cat-file", "-p", "ce01362"))
	_, errOut, code := s.run("cat-file", "ce01362")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "exactly one of")
}

func TestLsFilesStage(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("hello.txt", "hello\n")
	s.ok("add", ".")
	assert.Equal(t, "100644 ce013625030ba8dba906f756967f9e9ca394464a 0\thello.txt\n", s.ok("ls-files", "--stage"))
}

func TestConfig(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.ok("config", "user.name", "Grace")
	assert.Equal(t, "Grace\n", s.ok("config", "user.name"))
	_, errOut, code := s.run("config", "user.nothing")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "is not set")
}

func TestInitInDirectoryAndWithBranchName(t *testing.T) {
	s := newSession(t)
	s.ok("init", "-b", "trunk", "proj")
	_, _, code := s.run("init", "proj")
	assert.Equal(t, 1, code, "second init must fail")
	s.dir = filepath.Join(s.dir, "proj")
	assert.Contains(t, s.ok("status"), "On branch trunk")
}

func TestGlobalDirectoryFlag(t *testing.T) {
	s := newSession(t)
	s.ok("init", "proj")
	out := s.ok("-C", filepath.Join(s.dir, "proj"), "status")
	assert.Contains(t, out, "On branch main")
}

func TestStatusShowsModifiedAndDeleted(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1")
	s.write("b", "1")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.write("a", "2")
	require.NoError(t, os.Remove(filepath.Join(s.dir, "b")))
	out := s.ok("status")
	assert.Contains(t, out, "\tmodified:   a")
	assert.Contains(t, out, "\tdeleted:    b")
	assert.Contains(t, out, "no changes added to commit")
}

func TestDiffCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a.txt", "one\ntwo\n")
	s.write("b.txt", "keep\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "first")
	s.write("a.txt", "one\nTWO\n")
	s.write("b.txt", "keep\nmore\n")

	out := s.ok("diff")
	assert.Contains(t, out, "diff --git a/a.txt b/a.txt")
	assert.Contains(t, out, "-two\n+TWO\n")
	assert.Contains(t, out, "diff --git a/b.txt b/b.txt")

	out = s.ok("diff", "--", "b.txt")
	assert.NotContains(t, out, "a.txt")
	assert.Equal(t, out, s.ok("diff", "b.txt"), "a name that is not a revision is a path")

	assert.Equal(t, "a.txt\nb.txt\n", s.ok("diff", "--name-only"))
	assert.Equal(t, "M\ta.txt\nM\tb.txt\n", s.ok("diff", "--name-status"))
	assert.Empty(t, s.ok("diff", "--staged"))

	s.ok("add", "a.txt")
	assert.Contains(t, s.ok("diff", "--staged"), "+TWO")
	assert.NotContains(t, s.ok("diff"), "a.txt")
	assert.Equal(t, s.ok("diff", "--staged"), s.ok("diff", "--cached"))

	s.ok("commit", "-m", "second")
	out = s.ok("diff", "HEAD~1", "HEAD")
	assert.Contains(t, out, "-two\n+TWO\n")
}

func TestDiffContextFlag(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("f", "1\n2\n3\n4\n5\n6\n7\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "first")
	s.write("f", "1\n2\n3\nX\n5\n6\n7\n")
	assert.Contains(t, s.ok("diff", "-U", "1"), "@@ -3,3 +3,3 @@")
	_, errOut, code := s.run("diff", "-U", "-1")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "-U must not be negative")
}

func TestDiffErrors(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", "a")
	s.ok("commit", "-m", "first")
	_, errOut, code := s.run("diff", "--staged", "HEAD", "HEAD")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "--staged")
}

func TestBranchAndSwitchSession(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a.txt", "one\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "first")

	assert.Equal(t, "* main\n", s.ok("branch"))
	s.ok("branch", "feature")
	assert.Equal(t, "  feature\n* main\n", s.ok("branch"))
	assert.Regexp(t, `^  feature [0-9a-f]{7} first\n\* main [0-9a-f]{7} first\n$`, s.ok("branch", "-v"))

	assert.Equal(t, "Switched to branch 'feature'\n", s.ok("switch", "feature"))
	s.write("a.txt", "two\n")
	s.ok("commit", "-am", "second on feature")
	assert.Equal(t, "Switched to branch 'main'\n", s.ok("switch", "main"))
	data, _ := os.ReadFile(filepath.Join(s.dir, "a.txt"))
	assert.Equal(t, "one\n", string(data))
	assert.Equal(t, "Already on 'main'\n", s.ok("switch", "main"))

	assert.Equal(t, "Switched to a new branch 'topic'\n", s.ok("switch", "-c", "topic"))
	assert.Equal(t, "Switched to a new branch 'other'\n", s.ok("checkout", "-b", "other"))
	assert.Equal(t, "Switched to branch 'main'\n", s.ok("checkout", "main"))

	assert.Regexp(t, `^Deleted branch topic \(was [0-9a-f]{7}\)\.\n$`, s.ok("branch", "-d", "topic"))
	_, errOut, code := s.run("branch", "-d", "feature")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "not fully merged")
	s.ok("branch", "-D", "feature")
	s.ok("branch", "-m", "other", "renamed")
	assert.Contains(t, s.ok("branch"), "renamed")
	s.ok("branch", "-m", "trunk")
	assert.Contains(t, s.ok("status"), "On branch trunk")
}

func TestCheckoutDetachesAndBranchListShowsIt(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.write("a", "2\n")
	s.ok("commit", "-am", "two")

	out := s.ok("checkout", "HEAD~1")
	assert.Contains(t, out, "'detached HEAD' state")
	assert.Regexp(t, `HEAD is now at [0-9a-f]{7} one\n$`, out)
	assert.Regexp(t, `^\* \(HEAD detached at [0-9a-f]{7}\)\n  main\n$`, s.ok("branch"))
	assert.Regexp(t, `^HEAD detached at [0-9a-f]{7}\n`, s.ok("status"))
}

func TestSwitchRefusesToLoseChanges(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.ok("switch", "-c", "feature")
	s.write("a", "2\n")
	s.ok("commit", "-am", "two")
	s.write("a", "dirty\n")
	_, errOut, code := s.run("switch", "main")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "would be overwritten")
	assert.Contains(t, errOut, "a")
}

func TestTagCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")

	assert.Empty(t, s.ok("tag"))
	s.ok("tag", "v0.1")
	s.ok("tag", "-a", "-m", "release one", "v1.0")
	s.ok("tag", "-m", "notes", "v1.1")
	assert.Equal(t, "v0.1\nv1.0\nv1.1\n", s.ok("tag", "-l"))
	assert.Contains(t, s.ok("log", "--oneline"), "(HEAD -> main, tag: v1.1, tag: v1.0, tag: v0.1)")

	assert.Equal(t, "commit\n", s.ok("cat-file", "-t", "v0.1"))
	assert.Equal(t, "tag\n", s.ok("cat-file", "-t", "v1.0"))
	assert.Contains(t, s.ok("cat-file", "-p", "v1.0"), "tag v1.0\ntagger Ada Lovelace <ada@example.com>")

	assert.Regexp(t, `^Deleted tag 'v0.1' \(was [0-9a-f]{7}\)\n$`, s.ok("tag", "-d", "v0.1"))
	_, errOut, code := s.run("tag", "v1.0")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "tag already exists")
	_, errOut, code = s.run("tag", "-a", "v2")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "needs a message")
}

func TestRealGitReadsOurTags(t *testing.T) {
	testutil.RequireGit(t)
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.ok("tag", "light")
	s.ok("tag", "-m", "annotated message", "heavy")

	git := func(args ...string) string {
		return testutil.Git(t, s.dir, "", append([]string{"--git-dir=" + filepath.Join(s.dir, ".hood")}, args...)...)
	}
	git("fsck", "--strict")
	assert.Equal(t, "heavy\nlight\n", git("tag", "-l"))
	assert.Equal(t, "tag\n", git("cat-file", "-t", "heavy"))
	assert.Equal(t, "commit\n", git("cat-file", "-t", "heavy^{}"))
	assert.Contains(t, git("tag", "-n1", "heavy"), "annotated message")
}

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

func TestShowCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a.txt", "one\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "first")
	s.write("a.txt", "two\n")
	s.ok("commit", "-am", "second")
	s.ok("tag", "-m", "release", "v1")

	out := s.ok("show")
	assert.Contains(t, out, "(HEAD -> main, tag: v1)")
	assert.Contains(t, out, "    second\n\ndiff --git a/a.txt b/a.txt")
	assert.Contains(t, out, "-one\n+two\n")

	out = s.ok("show", "HEAD~1")
	assert.Contains(t, out, "new file mode 100644")

	out = s.ok("show", "v1")
	assert.True(t, strings.HasPrefix(out, "tag v1\nTagger: Ada Lovelace <ada@example.com>\n"))
	assert.Contains(t, out, "release\n")
	assert.Contains(t, out, "    second\n")

	assert.Equal(t, "a.txt\n", strings.SplitN(s.ok("show", "HEAD^{tree}"), "\n\n", 2)[1])
	assert.Equal(t, "two\n", s.ok("show", firstBlobPrefix(t, s)))
	_, _, code := s.run("show", "nope")
	assert.Equal(t, 1, code)
}

func firstBlobPrefix(t *testing.T, s *session) string {
	t.Helper()
	line := strings.TrimSpace(s.ok("ls-files", "--stage"))
	return strings.Fields(line)[1]
}

func TestRmCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.write("d/x", "x\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "first")

	assert.Equal(t, "rm 'a'\n", s.ok("rm", "a"))
	_, err := os.Stat(filepath.Join(s.dir, "a"))
	assert.True(t, os.IsNotExist(err))
	assert.Contains(t, s.ok("status"), "\tdeleted:    a")

	_, errOut, code := s.run("rm", "d")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "not removing a directory without -r")
	assert.Equal(t, "rm 'd/x'\n", s.ok("rm", "-r", "--cached", "d"))
	_, errOut, code = s.run("rm")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "usage: rm")
}

func TestResetCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	for _, v := range []string{"v1", "v2", "v3"} {
		s.write("a", v+"\n")
		s.ok("add", ".")
		s.ok("commit", "-m", v)
	}
	read := func() string {
		data, _ := os.ReadFile(filepath.Join(s.dir, "a"))
		return string(data)
	}

	s.ok("reset", "--soft", "HEAD~1")
	assert.Contains(t, s.ok("status"), "Changes to be committed:")
	assert.Equal(t, "v3\n", read())

	s.ok("reset")
	assert.Contains(t, s.ok("status"), "Changes not staged for commit:")

	out := s.ok("reset", "--hard", "HEAD~1")
	assert.Regexp(t, `^HEAD is now at [0-9a-f]{7} v1\n$`, out)
	assert.Equal(t, "v1\n", read())

	s.ok("reset", "--hard", "ORIG_HEAD")
	assert.Equal(t, "v2\n", read())
}

func TestResetPathsCommand(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.write("a", "2\n")
	s.write("b", "new\n")
	s.ok("add", ".")

	assert.Equal(t, "Unstaged changes after reset:\nM\ta\nM\tb\n", s.ok("reset", "a", "b"))
	out := s.ok("status")
	assert.NotContains(t, out, "Changes to be committed")
	assert.Contains(t, out, "\tmodified:   a")

	s.ok("add", "a")
	s.ok("reset", "HEAD", "--", "a")
	assert.NotContains(t, s.ok("status"), "Changes to be committed")

	_, errOut, code := s.run("reset", "--hard", "a")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "cannot be combined with paths")
	_, errOut, code = s.run("reset", "--soft", "--hard")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "only one of")
}

func TestMergeSessionFastForwardThenThreeWayThenConflict(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("story.txt", "intro\nmiddle\nend\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "base")
	s.ok("branch", "other")

	// Fast-forward.
	s.ok("switch", "-c", "ff")
	s.write("extra.txt", "x\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "extra")
	s.ok("switch", "main")
	out := s.ok("merge", "ff")
	assert.Regexp(t, `^Updating [0-9a-f]{7}\.\.[0-9a-f]{7}\nFast-forward\n$`, out)
	assert.Equal(t, "Already up to date.\n", s.ok("merge", "ff"))

	// Three-way merge, no conflict.
	s.ok("switch", "-c", "docs")
	s.write("README", "hi\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "docs")
	s.ok("switch", "main")
	s.write("story.txt", "intro\nMAIN\nend\n")
	s.ok("commit", "-am", "main edit")
	assert.Equal(t, "Merge made by the 'ort' strategy.\n", s.ok("merge", "docs"))
	assert.Contains(t, s.ok("log", "--oneline", "-n", "1"), "Merge branch 'docs'")

	// Conflict.
	s.ok("switch", "other")
	s.write("story.txt", "intro\nOTHER\nend\n")
	s.ok("commit", "-am", "other edit")
	s.ok("switch", "main")
	out, _, code := s.run("merge", "other")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "CONFLICT (content): Merge conflict in story.txt")
	assert.Contains(t, out, "Automatic merge failed; fix conflicts and then commit the result.")
	status := s.ok("status")
	assert.Contains(t, status, "Unmerged paths:")
	assert.Contains(t, status, "both modified:   story.txt")
	data, _ := os.ReadFile(filepath.Join(s.dir, "story.txt"))
	assert.Equal(t, "intro\n<<<<<<< HEAD\nMAIN\n=======\nOTHER\n>>>>>>> other\nend\n", string(data))

	_, errOut, code := s.run("commit", "-m", "too early")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "unmerged paths")

	s.write("story.txt", "intro\nBOTH\nend\n")
	s.ok("add", "story.txt")
	out = s.ok("commit")
	assert.Regexp(t, `^\[main [0-9a-f]{7}\] Merge branch 'other'\n$`, out)
	assert.Equal(t, "On branch main\nnothing to commit, working tree clean\n", s.ok("status"))
}

func TestMergeAbortAndErrors(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "base")
	s.ok("switch", "-c", "side")
	s.write("a", "side\n")
	s.ok("commit", "-am", "side")
	s.ok("switch", "main")
	s.write("a", "main\n")
	s.ok("commit", "-am", "main")

	_, _, code := s.run("merge", "side")
	assert.Equal(t, 1, code)
	s.ok("merge", "--abort")
	assert.Contains(t, s.ok("status"), "nothing to commit, working tree clean")
	_, errOut, code := s.run("merge", "--abort")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no merge in progress")

	_, errOut, code = s.run("merge", "--ff-only", "side")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "not possible to fast-forward")
	_, errOut, code = s.run("merge")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "usage: merge")
	_, _, code = s.run("merge", "--no-ff", "--ff-only", "side")
	assert.Equal(t, 2, code)
}

func TestModifyDeleteConflictMessage(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.write("keep", "k\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "base")
	s.ok("switch", "-c", "side")
	s.write("a", "edited\n")
	s.ok("commit", "-am", "edit")
	s.ok("switch", "main")
	s.ok("rm", "a")
	s.ok("commit", "-m", "delete")
	out, _, code := s.run("merge", "side")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "CONFLICT (modify/delete): a deleted in HEAD and modified in side. Version side of a left in tree.")
	assert.Contains(t, s.ok("status"), "deleted by us:   a")
}

func TestCherryPickAndRevertCommands(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "base")
	s.ok("switch", "-c", "feature")
	s.write("picked", "p\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "add picked")
	s.ok("switch", "main")

	out := s.ok("cherry-pick", "feature")
	assert.Regexp(t, `^\[main [0-9a-f]{7}\] add picked\n$`, out)
	assert.FileExists(t, filepath.Join(s.dir, "picked"))

	out = s.ok("revert", "HEAD")
	assert.Regexp(t, `^\[main [0-9a-f]{7}\] Revert "add picked"\n$`, out)
	assert.NoFileExists(t, filepath.Join(s.dir, "picked"))

	_, errOut, code := s.run("cherry-pick")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "cherry-pick needs one commit")
	_, errOut, code = s.run("revert", "--continue")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no cherry-pick or revert in progress")
}

func TestRevertConflictContinueAndAbort(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	for _, v := range []string{"1", "2", "3"} {
		s.write("a", v+"\n")
		s.ok("add", ".")
		s.ok("commit", "-m", "v"+v)
	}
	out, _, code := s.run("revert", "HEAD~1")
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "CONFLICT (content): Merge conflict in a")
	assert.Contains(t, out, "revert --continue")
	s.ok("revert", "--abort")
	assert.Contains(t, s.ok("status"), "nothing to commit, working tree clean")

	_, _, code = s.run("revert", "HEAD~1")
	assert.Equal(t, 1, code)
	s.write("a", "resolved\n")
	s.ok("add", "a")
	assert.Regexp(t, `^\[main [0-9a-f]{7}\] Revert "v2"\n$`, s.ok("revert", "--continue"))
}

func TestStashCommands(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	s.write("a", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "base")

	_, errOut, code := s.run("stash")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no local changes to save")

	s.write("a", "2\n")
	assert.Regexp(t, `^Saved working directory and index state WIP on main: [0-9a-f]{7} base\n$`, s.ok("stash"))
	assert.Contains(t, s.ok("status"), "nothing to commit, working tree clean")
	assert.Regexp(t, `^stash@\{0\}: WIP on main: [0-9a-f]{7} base\n$`, s.ok("stash", "list"))
	assert.Contains(t, s.ok("stash", "show"), "-1\n+2\n")
	assert.Contains(t, s.ok("diff", "HEAD", "stash@{0}"), "-1\n+2\n")

	out := s.ok("stash", "apply")
	assert.Contains(t, out, "\tmodified:   a")
	s.ok("reset", "--hard")
	out = s.ok("stash", "pop")
	assert.Regexp(t, `Dropped stash@\{0\} \([0-9a-f]{40}\)\n$`, out)
	assert.Empty(t, s.ok("stash", "list"))

	s.write("a", "3\n")
	s.ok("stash", "push", "-m", "wip three")
	assert.Equal(t, "stash@{0}: On main: wip three\n", s.ok("stash", "list"))
	assert.Regexp(t, `^Dropped stash@\{0\} \(`, s.ok("stash", "drop"))

	_, errOut, code = s.run("stash", "frobnicate")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "unknown stash command")
	_, _, code = s.run("stash", "pop", "nope")
	assert.Equal(t, 2, code)
	_, errOut, code = s.run("stash", "pop")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no stash entry found")
}

func TestReflogCommandAndRecovery(t *testing.T) {
	s := newSession(t)
	s.ok("init")
	for _, v := range []string{"one", "two", "three"} {
		s.write("f", v+"\n")
		s.ok("add", ".")
		s.ok("commit", "-m", v)
	}
	s.ok("reset", "--hard", "HEAD~2")

	lines := strings.Split(strings.TrimSpace(s.ok("reflog")), "\n")
	require.Len(t, lines, 4)
	assert.Regexp(t, `^[0-9a-f]{7} HEAD@\{0\}: reset: moving to HEAD~2$`, lines[0])
	assert.Regexp(t, `^[0-9a-f]{7} HEAD@\{1\}: commit: three$`, lines[1])
	assert.Regexp(t, `^[0-9a-f]{7} HEAD@\{3\}: commit \(initial\): one$`, lines[3])
	assert.Equal(t, s.ok("reflog"), s.ok("reflog", "show", "HEAD"))
	assert.Contains(t, s.ok("reflog", "main"), "main@{1}: commit: three")

	s.ok("reset", "--hard", "HEAD@{1}")
	data, _ := os.ReadFile(filepath.Join(s.dir, "f"))
	assert.Equal(t, "three\n", string(data))

	_, errOut, code := s.run("reflog", "ghost")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "branch not found")
}

func TestRealGitReadsOurReflog(t *testing.T) {
	testutil.RequireGit(t)
	s := newSession(t)
	s.ok("init")
	s.write("f", "1\n")
	s.ok("add", ".")
	s.ok("commit", "-m", "one")
	s.ok("switch", "-c", "topic")
	s.write("f", "2\n")
	s.ok("commit", "-am", "two")
	s.ok("reset", "--hard", "HEAD~1")

	git := func(args ...string) string {
		return testutil.Git(t, s.dir, "", append([]string{"--git-dir=" + filepath.Join(s.dir, ".hood")}, args...)...)
	}
	ourLines := strings.Split(strings.TrimSpace(s.ok("reflog")), "\n")
	realLines := strings.Split(strings.TrimSpace(git("reflog", "--no-abbrev-commit", "--format=%h %gd: %gs")), "\n")
	require.Len(t, realLines, len(ourLines))
	for i := range ourLines {
		assert.Equal(t, ourLines[i][8:], realLines[i][8:], "line %d", i)
	}
}

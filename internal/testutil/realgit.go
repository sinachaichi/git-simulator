package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// RequireGit skips the test when no git binary is on the PATH. The
// differential tests use real git only as a reference to compare with.
func RequireGit(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed, skipping differential test")
	}
}

// Git runs real git in dir with a clean environment and returns stdout.
// Identity and dates are fixed so output is reproducible.
func Git(t testing.TB, dir string, stdin string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, stdin, t.TempDir(), args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

// GitAllowDiff is like Git but accepts exit code 1, which `git diff
// --no-index` uses to say "the files differ".
func GitAllowDiff(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, "", t.TempDir(), args...)
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func runGit(dir, stdin, home string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Ada Lovelace",
		"GIT_AUTHOR_EMAIL=ada@example.com",
		"GIT_COMMITTER_NAME=Ada Lovelace",
		"GIT_COMMITTER_EMAIL=ada@example.com",
		"GIT_AUTHOR_DATE=1700000000 +0000",
		"GIT_COMMITTER_DATE=1700000000 +0000",
		"LC_ALL=C",
	)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%w\n%s", err, errBuf.String())
	}
	return out.String(), nil
}

// BareRepo creates an empty bare git repository and returns its path.
func BareRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	Git(t, dir, "", "init", "--bare", "-q", ".")
	return dir
}

// GitExit runs git and returns its output and exit code, for commands
// that use the exit code to report a result, like `merge-file`.
func GitExit(t testing.TB, dir string, args ...string) (string, int) {
	t.Helper()
	out, err := runGit(dir, "", t.TempDir(), args...)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out, 0
	case errors.As(err, &exit):
		return out, exit.ExitCode()
	}
	t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	return "", -1
}

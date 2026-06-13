package testutil

import (
	"bytes"
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
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+t.TempDir(),
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
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errBuf.String())
	}
	return out.String()
}

// BareRepo creates an empty bare git repository and returns its path.
func BareRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	Git(t, dir, "", "init", "--bare", "-q", ".")
	return dir
}

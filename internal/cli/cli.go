// Package cli parses the command line and runs commands. It is the only
// place that prints output.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/term"
)

// Env is everything a run needs from the outside world. Tests pass
// buffers, main passes the real streams.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Cwd    string
}

// Context is what a command handler gets.
type Context struct {
	Env
	Style term.Style
}

type command struct {
	name    string
	summary string
	usage   string
	run     func(c *Context, args []string) error
}

func commandTable() []command {
	return []command{
		{"init", "create an empty repository", "init [dir]", cmdInit},
		{"add", "stage files", "add <path>...", cmdAdd},
		{"reset", "move HEAD, or unstage files", "reset [--soft|--mixed|--hard] [<rev>] | reset [<rev>] [--] <path>...", cmdReset},
		{"rm", "remove files from the index and the working tree", "rm [--cached] [-f] [-r] <path>...", cmdRm},
		{"commit", "record the staged changes", "commit -m <message> [-a] [--amend] [--allow-empty]", cmdCommit},
		{"status", "show the state of the working tree and index", "status", cmdStatus},
		{"branch", "list, create, rename or delete branches", "branch [-v] | branch <name> [<start>] | branch -d|-D <name>... | branch -m [<old>] <new>", cmdBranch},
		{"switch", "switch to a branch", "switch [--detach] <branch> | switch -c <name> [<start>]", cmdSwitch},
		{"checkout", "switch branches or detach HEAD at a commit", "checkout <branch|commit> | checkout -b <name> [<start>]", cmdCheckout},
		{"show", "show a commit, tag, tree or blob", "show [<object>]", cmdShow},
		{"tag", "list, create or delete tags", "tag [-l] | tag <name> [<commit>] | tag -a -m <message> <name> [<commit>] | tag -d <name>...", cmdTag},
		{"diff", "show changes between commits, index and working tree", "diff [--staged] [<rev> [<rev>]] [-- <path>...]", cmdDiff},
		{"log", "show commit history", "log [--oneline] [-n <count>] [--all] [<rev>...]", cmdLog},
		{"cat-file", "show an object", "cat-file (-t | -s | -p) <object>", cmdCatFile},
		{"hash-object", "compute an object id", "hash-object [-w] <file>", cmdHashObject},
		{"ls-files", "list files in the index", "ls-files [--stage]", cmdLsFiles},
		{"config", "read or write repository settings", "config <section.key> [<value>]", cmdConfig},
	}
}

func findCommand(name string) (command, bool) {
	for _, c := range commandTable() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// Run executes one command line and returns the exit code.
func Run(args []string, env Env, version string) int {
	var noColor bool
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--no-color":
			noColor = true
			args = args[1:]
		case "-C":
			if len(args) < 2 {
				fmt.Fprintln(env.Stderr, "error: -C needs a directory")
				return 2
			}
			env.Cwd = args[1]
			args = args[2:]
		case "--version":
			fmt.Fprintln(env.Stdout, "hood "+version)
			return 0
		case "-h", "--help":
			printHelp(env.Stdout)
			return 0
		default:
			fmt.Fprintf(env.Stderr, "error: unknown option %s\n", args[0])
			return 2
		}
	}
	if len(args) == 0 {
		printHelp(env.Stdout)
		return 0
	}
	if env.Cwd == "" {
		env.Cwd, _ = os.Getwd()
	}
	ctx := &Context{Env: env, Style: term.Detect(env.Stdout, noColor)}

	cmd, ok := findCommand(args[0])
	if !ok {
		fmt.Fprintf(env.Stderr, "error: %q is not a command. Run `hood --help` to see the list.\n", args[0])
		return 2
	}
	if err := cmd.run(ctx, args[1:]); err != nil {
		var usage *usageError
		if errors.As(err, &usage) {
			fmt.Fprintf(env.Stderr, "error: %s\nusage: %s\n", err, cmd.usage)
			return 2
		}
		fmt.Fprintf(env.Stderr, "error: %s\n", err)
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "hood: a tiny git simulator that shows what happens under the hood")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "usage: hood [-C <dir>] [--no-color] <command> [<args>]")
	fmt.Fprintln(w)
	cmds := commandTable()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })
	for _, c := range cmds {
		fmt.Fprintf(w, "  %-12s %s\n", c.name, c.summary)
	}
}

// openRepo finds the repository that contains the current directory.
func (c *Context) openRepo() (*repo.Repo, error) {
	r, err := repo.Discover(c.Cwd)
	if errors.Is(err, repo.ErrNotARepo) {
		return nil, errors.New("not inside a hood repository. Run `hood init` first")
	}
	return r, err
}

// repoPath turns a path given on the command line (relative to the
// current directory) into a slash path relative to the repo root.
func (c *Context) repoPath(r *repo.Repo, p string) (string, error) {
	abs := p
	if !filepath.IsAbs(p) {
		abs = filepath.Join(c.Cwd, p)
	}
	rel, err := filepath.Rel(r.Root, filepath.Clean(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the repository", p)
	}
	return filepath.ToSlash(rel), nil
}

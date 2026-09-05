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

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/friendly"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/term"
	"github.com/sinachaichi/git-simulator/internal/xray"
)

// Env is everything a run needs from the outside world. Tests pass
// buffers, main passes the real streams.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Cwd    string
	// ChangeDir, when set, is called by commands that move the user to
	// another directory, such as `lesson start`. The shell uses it.
	ChangeDir func(dir string)
}

// Context is what a command handler gets.
type Context struct {
	Env
	Style term.Style

	// Xray turns on the summary of internal changes after each command.
	Xray     bool
	recorder events.Recorder
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
		{"lesson", "guided exercises that check your repository", "lesson list | lesson start <name> | lesson check | lesson hint | lesson status | lesson reset", cmdLesson},
		{"merge", "join another branch into the current one", "merge [--no-ff|--ff-only] [--no-commit] [-m <message>] <branch> | merge --abort", cmdMerge},
		{"cherry-pick", "apply the change of an existing commit", "cherry-pick [-n] <commit> | cherry-pick --continue | cherry-pick --abort", cmdCherryPick},
		{"revert", "make a new commit that undoes an earlier one", "revert [-n] <commit> | revert --continue | revert --abort", cmdRevert},
		{"stash", "put local changes aside", "stash [push [-m <message>]] | stash list | stash show|apply|pop|drop [stash@{<n>}]", cmdStash},
		{"reflog", "show where HEAD and branches used to point", "reflog [show] [<ref>]", cmdReflog},
		{"rm", "remove files from the index and the working tree", "rm [--cached] [-f] [-r] <path>...", cmdRm},
		{"commit", "record the staged changes", "commit -m <message> [-a] [--amend] [--allow-empty]", cmdCommit},
		{"status", "show the state of the working tree and index", "status", cmdStatus},
		{"branch", "list, create, rename or delete branches", "branch [-v] | branch <name> [<start>] | branch -d|-D <name>... | branch -m [<old>] <new>", cmdBranch},
		{"switch", "switch to a branch", "switch [--detach] <branch> | switch -c <name> [<start>]", cmdSwitch},
		{"checkout", "switch branches or detach HEAD at a commit", "checkout <branch|commit> | checkout -b <name> [<start>]", cmdCheckout},
		{"show", "show a commit, tag, tree or blob", "show [<object>]", cmdShow},
		{"tag", "list, create or delete tags", "tag [-l] | tag <name> [<commit>] | tag -a -m <message> <name> [<commit>] | tag -d <name>...", cmdTag},
		{"graph", "draw all branches as a commit graph", "graph [-n <count>]", cmdGraph},
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
	xrayOn := os.Getenv("HOOD_XRAY") != "" && os.Getenv("HOOD_XRAY") != "0"
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--no-color":
			noColor = true
			args = args[1:]
		case "--xray":
			xrayOn = true
			args = args[1:]
		case "--no-xray":
			xrayOn = false
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
	ctx := &Context{Env: env, Style: term.Detect(env.Stdout, noColor), Xray: xrayOn}

	cmd, ok := findCommand(args[0])
	if !ok {
		fmt.Fprintf(env.Stderr, "error: %q is not a command. Run `hood --help` to see the list.\n", args[0])
		return 2
	}
	err := cmd.run(ctx, args[1:])
	ctx.printXray()
	if err != nil {
		if errors.Is(err, errConflicts) {
			return 1
		}
		var usage *usageError
		if errors.As(err, &usage) {
			fmt.Fprintf(env.Stderr, "error: %s\nusage: %s\n", err, cmd.usage)
			return 2
		}
		fmt.Fprintf(env.Stderr, "error: %s\n", err)
		if help, ok := friendly.For(err); ok {
			fmt.Fprintf(env.Stderr, "  why: %s\n  try: %s\n", help.Why, help.Try)
		}
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "hood: a tiny git simulator that shows what happens under the hood")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "usage: hood [-C <dir>] [--no-color] [--xray] <command> [<args>]")
	fmt.Fprintln(w)
	cmds := commandTable()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })
	for _, c := range cmds {
		fmt.Fprintf(w, "  %-12s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, "  %-12s %s\n", "shell", "start the interactive shell (also what plain `hood` does)")
}

// openRepo finds the repository that contains the current directory.
func (c *Context) openRepo() (*repo.Repo, error) {
	r, err := repo.Discover(c.Cwd)
	if err == nil && c.Xray {
		r.SetObserver(&c.recorder)
	}
	return r, err
}

// printXray shows what the command did inside the repository, when
// X-ray mode is on.
func (c *Context) printXray() {
	if !c.Xray {
		return
	}
	xray.Render(c.Stdout, c.Style, c.recorder.Events())
	c.recorder.Reset()
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

// Info describes a command for help texts and shell completion.
type Info struct {
	Name    string
	Summary string
	Usage   string
}

// Commands lists the commands, sorted by name.
func Commands() []Info {
	cmds := commandTable()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })
	out := make([]Info, len(cmds))
	for i, c := range cmds {
		out[i] = Info{Name: c.name, Summary: c.summary, Usage: c.usage}
	}
	return out
}

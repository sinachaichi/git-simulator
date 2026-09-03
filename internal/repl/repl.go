package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/lessons"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// Shell is the interactive session.
type Shell struct {
	env     cli.Env
	version string
	cwd     string
	xray    bool
	live    bool
}

// Run starts the shell on the given streams and returns an exit code.
// On a terminal it uses line editing with history and completion, and
// otherwise it reads plain lines, so it also works with a pipe.
func Run(env cli.Env, version string) int {
	cwd := env.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	s := &Shell{env: env, version: version, cwd: cwd, xray: true}

	var in lineReader
	if isTerminal(env.Stdin) {
		in = newTerminalReader(s)
	} else {
		in = &plainReader{r: bufio.NewReader(env.Stdin), out: env.Stdout}
	}
	defer in.Close()

	fmt.Fprintf(env.Stdout, "hood %s. Type `help` for commands, `lesson list` for exercises.\n", version)
	fmt.Fprintln(env.Stdout, "X-ray is on: after each command you see what happened inside. Turn it off with `xray off`.")
	for {
		line, err := in.ReadLine(s.prompt())
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(env.Stdout)
			return 0
		}
		if errors.Is(err, liner.ErrPromptAborted) {
			continue
		}
		if err != nil {
			fmt.Fprintf(env.Stderr, "error: %v\n", err)
			return 1
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		in.AddHistory(line)
		if err := s.execute(line); errors.Is(err, errExit) {
			return 0
		}
	}
}

// prompt shows the branch of the repository we are in, if any.
func (s *Shell) prompt() string {
	r, err := repo.Discover(s.cwd)
	if err != nil {
		return "hood> "
	}
	head, err := r.Refs.Head()
	switch {
	case err != nil:
		return "hood> "
	case head.Detached():
		return fmt.Sprintf("hood (%s)> ", head.ID.Short())
	}
	return fmt.Sprintf("hood (%s)> ", head.Branch())
}

// execute runs one line. Errors are printed here, so only errExit leaves.
func (s *Shell) execute(line string) error {
	args, err := SplitArgs(line)
	if err != nil {
		fmt.Fprintf(s.env.Stderr, "error: %v\n", err)
		return nil
	}
	// Habits from the real thing: `git status` and `hood status` work too.
	if len(args) > 0 && (args[0] == "git" || args[0] == "hood") {
		args = args[1:]
	}
	if len(args) == 0 {
		return nil
	}
	if b, ok := findBuiltin(args[0]); ok {
		err := b.run(s, args[1:])
		if errors.Is(err, errExit) {
			return err
		}
		if err != nil {
			fmt.Fprintf(s.env.Stderr, "error: %v\n", err)
		}
		return nil
	}

	env := s.env
	env.Cwd = s.cwd
	env.ChangeDir = func(dir string) { s.cwd = dir }
	if s.xray {
		args = append([]string{"--xray"}, args...)
	}
	cli.Run(args, env, s.version)
	if s.live {
		cli.Run([]string{"graph"}, env, s.version)
	}
	return nil
}

// homeDir is where history is kept.
func homeDir() string {
	home, err := lessons.DefaultHome()
	if err != nil {
		return ""
	}
	return home
}

func historyFile() string {
	if h := homeDir(); h != "" {
		return filepath.Join(h, "history")
	}
	return ""
}

package repl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/workdir"
)

// errExit is returned by `exit` to stop the shell.
var errExit = errors.New("exit")

type builtin struct {
	name    string
	summary string
	usage   string
	run     func(s *Shell, args []string) error
}

func builtins() []builtin {
	return []builtin{
		{"help", "list commands, or show the usage of one", "help [<command>]", (*Shell).help},
		{"exit", "leave the shell", "exit", func(*Shell, []string) error { return errExit }},
		{"quit", "leave the shell", "quit", func(*Shell, []string) error { return errExit }},
		{"xray", "show or change X-ray mode", "xray [on|off]", func(s *Shell, a []string) error { return s.toggle("x-ray", &s.xray, a) }},
		{"live", "redraw the graph after every command", "live [on|off]", func(s *Shell, a []string) error { return s.toggle("live graph", &s.live, a) }},
		{"pwd", "print the current directory", "pwd", (*Shell).pwd},
		{"cd", "change the current directory", "cd <dir>", (*Shell).cd},
		{"ls", "list files in the working directory", "ls [<dir>]", (*Shell).ls},
		{"cat", "print a file", "cat <file>", (*Shell).cat},
		{"write", "replace a file with the given text", "write <file> <text>...", (*Shell).write},
		{"append", "add a line to a file", "append <file> <text>...", (*Shell).appendLine},
		{"del", "delete a file without telling hood", "del <file>", (*Shell).del},
	}
}

func findBuiltin(name string) (builtin, bool) {
	for _, b := range builtins() {
		if b.name == name {
			return b, true
		}
	}
	return builtin{}, false
}

func (s *Shell) help(args []string) error {
	if len(args) == 1 {
		if b, ok := findBuiltin(args[0]); ok {
			fmt.Fprintf(s.env.Stdout, "%s\n  %s\n", b.usage, b.summary)
			return nil
		}
		for _, c := range cli.Commands() {
			if c.Name == args[0] {
				fmt.Fprintf(s.env.Stdout, "%s\n  %s\n", c.Usage, c.Summary)
				return nil
			}
		}
		return fmt.Errorf("no help for %q", args[0])
	}
	fmt.Fprintln(s.env.Stdout, "hood commands:")
	for _, c := range cli.Commands() {
		fmt.Fprintf(s.env.Stdout, "  %-12s %s\n", c.Name, c.Summary)
	}
	fmt.Fprintln(s.env.Stdout, "\nshell helpers:")
	bs := builtins()
	sort.Slice(bs, func(i, j int) bool { return bs[i].name < bs[j].name })
	for _, b := range bs {
		fmt.Fprintf(s.env.Stdout, "  %-12s %s\n", b.name, b.summary)
	}
	fmt.Fprintln(s.env.Stdout, "\nTry `lesson list` for guided exercises. Ctrl-D leaves the shell.")
	return nil
}

func (s *Shell) toggle(what string, flag *bool, args []string) error {
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "on":
		*flag = true
	case len(args) == 1 && args[0] == "off":
		*flag = false
	default:
		return fmt.Errorf("use `on` or `off`")
	}
	state := "off"
	if *flag {
		state = "on"
	}
	fmt.Fprintf(s.env.Stdout, "%s is %s\n", what, state)
	return nil
}

func (s *Shell) pwd([]string) error {
	fmt.Fprintln(s.env.Stdout, s.cwd)
	return nil
}

func (s *Shell) cd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: cd <dir>")
	}
	dir := args[0]
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(s.cwd, dir)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", args[0])
	}
	s.cwd = filepath.Clean(dir)
	return nil
}

// work opens the current directory as a working tree. File helpers use it
// so they cannot leave the directory or touch .hood.
func (s *Shell) work() (*workdir.WorkDir, error) { return workdir.New(s.cwd) }

func (s *Shell) ls(args []string) error {
	w, err := s.work()
	if err != nil {
		return err
	}
	dir := "."
	if len(args) > 1 {
		return errors.New("usage: ls [<dir>]")
	}
	if len(args) == 1 {
		dir = args[0]
	}
	files, err := w.ListIn(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		fmt.Fprintln(s.env.Stdout, f)
	}
	return nil
}

func (s *Shell) cat(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: cat <file>")
	}
	w, err := s.work()
	if err != nil {
		return err
	}
	data, err := w.ReadFile(args[0])
	if err != nil {
		return err
	}
	_, err = s.env.Stdout.Write(data)
	return err
}

func (s *Shell) write(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: write <file> <text>")
	}
	w, err := s.work()
	if err != nil {
		return err
	}
	return w.WriteFile(args[0], []byte(strings.Join(args[1:], " ")+"\n"), 0o644)
}

func (s *Shell) appendLine(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: append <file> <text>")
	}
	w, err := s.work()
	if err != nil {
		return err
	}
	line := []byte(strings.Join(args[1:], " ") + "\n")
	if !w.Exists(args[0]) {
		return w.WriteFile(args[0], line, 0o644)
	}
	data, err := w.ReadFile(args[0])
	if err != nil {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		line = append([]byte("\n"), line...)
	}
	return w.AppendFile(args[0], line)
}

func (s *Shell) del(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: del <file>")
	}
	w, err := s.work()
	if err != nil {
		return err
	}
	return w.Remove(args[0])
}

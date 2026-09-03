package repl

import (
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// complete returns the lines that the current input can be completed to.
// The first word is completed from the commands, the other words from
// branch names, tag names and files, whichever starts with the text.
func (s *Shell) complete(line string) []string {
	cut := strings.LastIndexAny(line, " \t") + 1
	head, word := line[:cut], line[cut:]

	var candidates []string
	if strings.TrimSpace(head) == "" {
		for _, c := range cli.Commands() {
			candidates = append(candidates, c.Name)
		}
		for _, b := range builtins() {
			candidates = append(candidates, b.name)
		}
	} else {
		candidates = s.argumentCandidates()
	}
	var out []string
	for _, c := range candidates {
		if strings.HasPrefix(c, word) && c != word {
			out = append(out, head+c)
		}
	}
	sort.Strings(out)
	return out
}

// argumentCandidates lists names that make sense after a command.
func (s *Shell) argumentCandidates() []string {
	var out []string
	r, err := repo.Discover(s.cwd)
	if err != nil {
		return out
	}
	for _, prefix := range []string{refs.BranchPrefix, refs.TagPrefix} {
		list, err := r.Refs.List(prefix)
		if err != nil {
			continue
		}
		for _, ref := range list {
			out = append(out, strings.TrimPrefix(ref.Name, prefix))
		}
	}
	if files, err := r.Work.List(); err == nil {
		out = append(out, files...)
	}
	return out
}

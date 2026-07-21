package cli

import (
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

func cmdDiff(c *Context, args []string) error {
	fs := newFlags("diff")
	staged := fs.Bool("staged", false, "compare the index with HEAD")
	fs.BoolVar(staged, "cached", false, "same as --staged")
	nameOnly := fs.Bool("name-only", false, "show only the names of changed files")
	nameStatus := fs.Bool("name-status", false, "show names and the kind of change")
	context := fs.Int("U", diff.DefaultContext, "lines of context")
	// Arguments after "--" are always paths.
	var dashPaths []string
	hasDashes := false
	for i, a := range args {
		if a == "--" {
			args, dashPaths, hasDashes = args[:i], args[i+1:], true
			break
		}
	}
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *context < 0 {
		return usagef("-U must not be negative")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	revs, specs, err := c.splitRevsAndPaths(r, rest, dashPaths, hasDashes)
	if err != nil {
		return err
	}
	files, err := porcelain.Diff(r, porcelain.DiffOptions{Staged: *staged, Revs: revs, Paths: specs})
	if err != nil {
		return err
	}
	for _, f := range files {
		switch {
		case *nameOnly:
			fmt.Fprintln(c.Stdout, f.Path)
		case *nameStatus:
			fmt.Fprintf(c.Stdout, "%s\t%s\n", statusLetter(f), f.Path)
		default:
			short := func(id objects.ID) string {
				s, err := r.ODB.Abbrev(id, 7)
				if err != nil {
					return id.Short()
				}
				return s
			}
			fmt.Fprint(c.Stdout, c.colorPatch(f.Render(short, *context)))
		}
	}
	return nil
}

// splitRevsAndPaths separates revisions from paths. After "--" everything
// is a path. Without it, arguments that name a revision are revisions (at
// most two) and the rest are paths, like git does.
func (c *Context) splitRevsAndPaths(r *repo.Repo, positional, afterDashes []string, hasDashes bool) (revs, paths []string, err error) {
	if hasDashes {
		revs, paths = positional, afterDashes
	} else {
		for _, a := range positional {
			if _, err := revparse.Resolve(r, a); err == nil && len(revs) < 2 {
				revs = append(revs, a)
			} else {
				paths = append(paths, a)
			}
		}
	}
	for i, p := range paths {
		if paths[i], err = c.repoPath(r, p); err != nil {
			return nil, nil, err
		}
	}
	return revs, paths, nil
}

func statusLetter(f diff.File) string {
	switch {
	case f.OldMode == 0:
		return "A"
	case f.NewMode == 0:
		return "D"
	}
	return "M"
}

// colorPatch colors a rendered patch the way git does by default.
func (c *Context) colorPatch(patch string) string {
	if !c.Style.Enabled {
		return patch
	}
	var b strings.Builder
	for _, line := range strings.SplitAfter(patch, "\n") {
		text := strings.TrimSuffix(line, "\n")
		nl := line[len(text):]
		switch {
		case strings.HasPrefix(text, "diff --git"), strings.HasPrefix(text, "index "),
			strings.HasPrefix(text, "--- "), strings.HasPrefix(text, "+++ "),
			strings.HasPrefix(text, "new file"), strings.HasPrefix(text, "deleted file"),
			strings.HasPrefix(text, "old mode"), strings.HasPrefix(text, "new mode"):
			text = c.Style.Bold(text)
		case strings.HasPrefix(text, "@@"):
			text = c.Style.Cyan(text)
		case strings.HasPrefix(text, "+"):
			text = c.Style.Green(text)
		case strings.HasPrefix(text, "-"):
			text = c.Style.Red(text)
		}
		b.WriteString(text + nl)
	}
	return b.String()
}

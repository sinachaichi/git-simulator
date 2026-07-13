package cli

import (
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// dateLayout is git's default log date format.
const dateLayout = "Mon Jan 2 15:04:05 2006 -0700"

func cmdLog(c *Context, args []string) error {
	fs := newFlags("log")
	oneline := fs.Bool("oneline", false, "one line per commit")
	all := fs.Bool("all", false, "start from all branches and tags")
	max := fs.Int("n", 0, "limit the number of commits")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *max < 0 {
		return usagef("-n must not be negative")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	nodes, err := porcelain.Log(r, porcelain.LogOptions{Revs: rest, All: *all, Max: *max})
	if err != nil {
		return err
	}
	decor, err := porcelain.Decorations(r)
	if err != nil {
		return err
	}
	for i, n := range nodes {
		label, err := c.decoration(decor[n.ID])
		if err != nil {
			return err
		}
		short, err := r.ODB.Abbrev(n.ID, 7)
		if err != nil {
			return err
		}
		if *oneline {
			fmt.Fprintf(c.Stdout, "%s%s %s\n", c.Style.Yellow(short), label, n.Commit.Subject())
			continue
		}
		if i > 0 {
			fmt.Fprintln(c.Stdout)
		}
		c.writeCommit(r, n, label)
	}
	return nil
}

// decoration renders " (HEAD -> main, tag: v1)" with the colors git uses.
func (c *Context) decoration(names []string) (string, error) {
	if len(names) == 0 {
		return "", nil
	}
	parts := make([]string, len(names))
	for i, n := range names {
		switch {
		case strings.HasPrefix(n, "HEAD"):
			parts[i] = c.Style.Bold(c.Style.Cyan(n))
		case strings.HasPrefix(n, "tag: "):
			parts[i] = c.Style.Bold(c.Style.Yellow(n))
		default:
			parts[i] = c.Style.Bold(c.Style.Green(n))
		}
	}
	return " (" + strings.Join(parts, ", ") + ")", nil
}

func (c *Context) writeCommit(_ *repo.Repo, n history.Node, label string) {
	fmt.Fprintf(c.Stdout, "%s%s\n", c.Style.Yellow("commit "+n.ID.String()), label)
	if len(n.Commit.Parents) > 1 {
		fmt.Fprintf(c.Stdout, "Merge: %s\n", strings.Join(shortIDs(n.Commit.Parents), " "))
	}
	fmt.Fprintf(c.Stdout, "Author: %s <%s>\n", n.Commit.Author.Name, n.Commit.Author.Email)
	fmt.Fprintf(c.Stdout, "Date:   %s\n\n", n.Commit.Author.When.Format(dateLayout))
	for _, line := range strings.Split(strings.TrimSuffix(n.Commit.Message, "\n"), "\n") {
		if line == "" {
			fmt.Fprintln(c.Stdout)
			continue
		}
		fmt.Fprintf(c.Stdout, "    %s\n", line)
	}
}

func shortIDs(ids []objects.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Short()
	}
	return out
}

package cli

import (
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/graph"
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
	showGraph := fs.Bool("graph", false, "draw the commit graph")
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
	if *showGraph {
		return c.logGraph(r, nodes, decor, *oneline)
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
	for _, line := range c.commitLines(n, label) {
		fmt.Fprintln(c.Stdout, line)
	}
}

// commitLines returns the lines of the default log format for one commit.
func (c *Context) commitLines(n history.Node, label string) []string {
	lines := []string{c.Style.Yellow("commit "+n.ID.String()) + label}
	if len(n.Commit.Parents) > 1 {
		lines = append(lines, "Merge: "+strings.Join(shortIDs(n.Commit.Parents), " "))
	}
	lines = append(lines,
		fmt.Sprintf("Author: %s <%s>", n.Commit.Author.Name, n.Commit.Author.Email),
		"Date:   "+n.Commit.Author.When.Format(dateLayout), "")
	for _, line := range strings.Split(strings.TrimSuffix(n.Commit.Message, "\n"), "\n") {
		if line == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, "    "+line)
	}
	return lines
}

// logGraph prints commits with the lane drawing in front, like
// `log --graph`.
func (c *Context) logGraph(r *repo.Repo, nodes []history.Node, decor map[objects.ID][]string, oneline bool) error {
	rows := graph.Layout(graph.TopoOrder(nodes))
	for i, row := range rows {
		label, err := c.decoration(decor[row.Node.ID])
		if err != nil {
			return err
		}
		prefix := graph.Text(c.Style, row.Prefix)
		var lines []string
		if oneline {
			short, err := r.ODB.Abbrev(row.Node.ID, 7)
			if err != nil {
				return err
			}
			lines = []string{c.Style.Yellow(short) + label + " " + row.Node.Commit.Subject()}
		} else {
			lines = c.commitLines(row.Node, label)
		}
		// Lines after the first sit under the lanes, so the star becomes a bar.
		cont := graph.Text(c.Style, graph.Continuation(row.Prefix))
		for k, l := range lines {
			p := cont
			if k == 0 {
				p = prefix
			}
			fmt.Fprintln(c.Stdout, strings.TrimRight(p+l, " "))
		}
		for _, between := range row.Between {
			fmt.Fprintln(c.Stdout, strings.TrimRight(graph.Text(c.Style, between), " "))
		}
		if !oneline && i < len(rows)-1 && len(row.Between) == 0 {
			fmt.Fprintln(c.Stdout, strings.TrimRight(graph.Text(c.Style, graph.Continuation(row.Prefix)), " "))
		}
	}
	return nil
}

func shortIDs(ids []objects.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Short()
	}
	return out
}

// cmdGraph is shorthand for `log --graph --oneline --all`: the whole
// repository as a picture.
func cmdGraph(c *Context, args []string) error {
	return cmdLog(c, append([]string{"--graph", "--oneline", "--all"}, args...))
}

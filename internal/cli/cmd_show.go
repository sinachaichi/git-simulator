package cli

import (
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func cmdShow(c *Context, args []string) error {
	fs := newFlags("show")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return usagef("show takes one object")
	}
	rev := "HEAD"
	if len(rest) == 1 {
		rev = rest[0]
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	res, err := porcelain.Show(r, rev)
	if err != nil {
		return err
	}
	return c.writeShow(r, res)
}

func (c *Context) writeShow(r *repo.Repo, res porcelain.ShowResult) error {
	switch {
	case res.Tag != nil:
		fmt.Fprintf(c.Stdout, "%s %s\n", c.Style.Yellow("tag"), res.Tag.Name)
		fmt.Fprintf(c.Stdout, "Tagger: %s <%s>\n", res.Tag.Tagger.Name, res.Tag.Tagger.Email)
		fmt.Fprintf(c.Stdout, "Date:   %s\n\n%s\n", res.Tag.Tagger.When.Format(dateLayout), strings.TrimSuffix(res.Tag.Message, "\n"))
		return c.writeShow(r, *res.Next)
	case res.Commit != nil:
		decor, err := porcelain.Decorations(r)
		if err != nil {
			return err
		}
		label, err := c.decoration(decor[res.ID])
		if err != nil {
			return err
		}
		c.writeCommit(r, history.Node{ID: res.ID, Commit: res.Commit}, label)
		short := func(id objects.ID) string {
			s, err := r.ODB.Abbrev(id, 7)
			if err != nil {
				return id.Short()
			}
			return s
		}
		for i, f := range res.Files {
			if i == 0 {
				fmt.Fprintln(c.Stdout)
			}
			fmt.Fprint(c.Stdout, c.colorPatch(f.Render(short, diff.DefaultContext)))
		}
	case res.Tree != nil:
		fmt.Fprintf(c.Stdout, "tree %s\n\n", res.ID)
		for _, e := range res.Tree.Entries {
			name := e.Name
			if e.IsDir() {
				name += "/"
			}
			fmt.Fprintln(c.Stdout, name)
		}
	case res.Blob != nil:
		_, err := c.Stdout.Write(res.Blob.Data)
		return err
	}
	return nil
}

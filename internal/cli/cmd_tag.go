package cli

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func cmdTag(c *Context, args []string) error {
	fs := newFlags("tag")
	annotate := fs.Bool("a", false, "make an annotated tag, needs -m")
	var messages stringList
	fs.Var(&messages, "m", "tag message, makes the tag annotated")
	del := fs.Bool("d", false, "delete a tag")
	list := fs.Bool("l", false, "list tags")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	switch {
	case *del:
		if len(rest) == 0 {
			return usagef("tag -d needs a tag name")
		}
		for _, name := range rest {
			id, err := porcelain.DeleteTag(r, name)
			if err != nil {
				return err
			}
			short, err := r.ODB.Abbrev(id, 7)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.Stdout, "Deleted tag '%s' (was %s)\n", name, short)
		}
		return nil
	case *list || len(rest) == 0:
		tags, err := porcelain.ListTags(r)
		if err != nil {
			return err
		}
		for _, t := range tags {
			fmt.Fprintln(c.Stdout, t.Name)
		}
		return nil
	case len(rest) > 2:
		return usagef("tag takes a name and an optional commit")
	}
	if *annotate && len(messages) == 0 {
		return usagef("an annotated tag needs a message, use -m")
	}
	rev := ""
	if len(rest) == 2 {
		rev = rest[1]
	}
	_, err = porcelain.CreateTag(r, rest[0], rev, joinMessages(messages))
	return err
}

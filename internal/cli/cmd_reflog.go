package cli

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func cmdReflog(c *Context, args []string) error {
	if len(args) > 0 && args[0] == "show" {
		args = args[1:]
	}
	fs := newFlags("reflog")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return usagef("reflog takes at most one ref")
	}
	ref := ""
	if len(rest) == 1 {
		ref = rest[0]
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	lines, err := porcelain.Reflog(r, ref)
	if err != nil {
		return err
	}
	for _, l := range lines {
		short, err := r.ODB.Abbrev(l.ID, 7)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "%s %s: %s\n", c.Style.Yellow(short), l.Selector, l.Message)
	}
	return nil
}

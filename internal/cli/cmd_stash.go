package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

func cmdStash(c *Context, args []string) error {
	sub := "push"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	switch sub {
	case "push", "save":
		fs := newFlags("stash push")
		var messages stringList
		fs.Var(&messages, "m", "stash message")
		rest, err := parseFlags(fs, args)
		if err != nil {
			return err
		}
		if len(rest) > 0 {
			return usagef("stash push takes no paths yet")
		}
		entry, err := porcelain.StashPush(r, joinMessages(messages))
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "Saved working directory and index state %s\n", entry.Message)
		return nil
	case "list":
		list, err := porcelain.StashList(r)
		if err != nil {
			return err
		}
		for _, e := range list {
			fmt.Fprintf(c.Stdout, "stash@{%d}: %s\n", e.Index, e.Message)
		}
		return nil
	case "apply", "pop", "drop", "show":
		n, err := stashIndex(args)
		if err != nil {
			return err
		}
		return c.stashEntryCommand(sub, n)
	}
	return usagef("unknown stash command %q (use push, list, apply, pop, drop or show)", sub)
}

// stashIndex reads an optional "stash@{n}" or "n" argument.
func stashIndex(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) > 1 {
		return 0, usagef("expected at most one stash entry")
	}
	text := strings.TrimSuffix(strings.TrimPrefix(args[0], "stash@{"), "}")
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0, usagef("%q is not a stash entry, use stash@{0}", args[0])
	}
	return n, nil
}

func (c *Context) stashEntryCommand(sub string, n int) error {
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	short := func(id objects.ID) string {
		s, err := r.ODB.Abbrev(id, 7)
		if err != nil {
			return id.Short()
		}
		return s
	}
	switch sub {
	case "show":
		files, err := porcelain.StashShow(r, n)
		if err != nil {
			return err
		}
		for _, f := range files {
			fmt.Fprint(c.Stdout, c.colorPatch(f.Render(short, diff.DefaultContext)))
		}
	case "drop":
		e, err := porcelain.StashDrop(r, n)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "Dropped stash@{%d} (%s)\n", n, e.ID)
	case "apply", "pop":
		var conflicts []porcelain.ConflictInfo
		var dropped porcelain.StashEntry
		if sub == "apply" {
			conflicts, err = porcelain.StashApply(r, n)
		} else {
			conflicts, dropped, err = porcelain.StashPop(r, n)
		}
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			for _, cf := range conflicts {
				fmt.Fprintln(c.Stdout, c.conflictLine(cf, "Stashed changes"))
			}
			fmt.Fprintln(c.Stdout, "The stash entry is kept in case you need it again.")
			return errConflicts
		}
		res, err := porcelain.Status(r)
		if err != nil {
			return err
		}
		writeStatus(c.Stdout, c.Style, res)
		if sub == "pop" {
			fmt.Fprintf(c.Stdout, "Dropped stash@{%d} (%s)\n", n, dropped.ID)
		}
	}
	return nil
}

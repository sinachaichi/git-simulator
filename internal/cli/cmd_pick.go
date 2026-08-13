package cli

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func cmdCherryPick(c *Context, args []string) error {
	return c.pickCommand("cherry-pick", args, porcelain.CherryPick)
}

func cmdRevert(c *Context, args []string) error {
	return c.pickCommand("revert", args, porcelain.Revert)
}

type pickFunc func(r *repo.Repo, rev string, opts porcelain.PickOptions) (porcelain.PickResult, error)

func (c *Context) pickCommand(name string, args []string, run pickFunc) error {
	fs := newFlags(name)
	noCommit := fs.Bool("n", false, "apply the change but do not commit")
	fs.BoolVar(noCommit, "no-commit", false, "same as -n")
	abort := fs.Bool("abort", false, "give up and go back to HEAD")
	cont := fs.Bool("continue", false, "commit the resolved result")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	switch {
	case *abort:
		return porcelain.PickAbort(r)
	case *cont:
		if !porcelain.PickInProgress(r) {
			return porcelain.ErrNoPick
		}
		return cmdCommit(c, nil)
	case len(rest) != 1:
		return usagef("%s needs one commit", name)
	}
	res, err := run(r, rest[0], porcelain.PickOptions{NoCommit: *noCommit})
	if err != nil {
		return err
	}
	switch res.Outcome {
	case porcelain.MergeCommitted:
		short, err := r.ODB.Abbrev(res.Commit, 7)
		if err != nil {
			return err
		}
		commit, err := r.ODB.ReadCommit(res.Commit)
		if err != nil {
			return err
		}
		head, err := r.Refs.Head()
		if err != nil {
			return err
		}
		where := head.Branch()
		if where == "" {
			where = "detached HEAD"
		}
		fmt.Fprintf(c.Stdout, "[%s %s] %s\n", where, short, commit.Subject())
	case porcelain.MergeStopped:
		fmt.Fprintln(c.Stdout, "Changes applied and staged, nothing committed")
	case porcelain.MergeConflicted:
		for _, cf := range res.Conflicts {
			fmt.Fprintln(c.Stdout, c.conflictLine(cf, res.Source.Short()))
		}
		fmt.Fprintf(c.Stdout, "error: could not apply %s\n", res.Source.Short())
		fmt.Fprintf(c.Stdout, "hint: fix the conflicts, run \"add <file>\" and then \"%s --continue\", or use --abort\n", name)
		return errConflicts
	}
	return nil
}

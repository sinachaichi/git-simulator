package cli

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func cmdBranch(c *Context, args []string) error {
	fs := newFlags("branch")
	del := fs.Bool("d", false, "delete a merged branch")
	forceDel := fs.Bool("D", false, "delete a branch even if it is not merged")
	rename := fs.Bool("m", false, "rename a branch")
	verbose := fs.Bool("v", false, "show the tip commit")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	switch {
	case *del || *forceDel:
		if len(rest) == 0 {
			return usagef("branch -d needs a branch name")
		}
		for _, name := range rest {
			id, err := porcelain.DeleteBranch(r, name, *forceDel)
			if err != nil {
				return err
			}
			short, err := r.ODB.Abbrev(id, 7)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.Stdout, "Deleted branch %s (was %s).\n", name, short)
		}
		return nil
	case *rename:
		return c.renameBranch(r, rest)
	case len(rest) == 0:
		return c.listBranches(r, *verbose)
	case len(rest) > 2:
		return usagef("branch takes a name and an optional start point")
	}
	start := ""
	if len(rest) == 2 {
		start = rest[1]
	}
	_, err = porcelain.CreateBranch(r, rest[0], start)
	return err
}

func (c *Context) renameBranch(r *repo.Repo, rest []string) error {
	switch len(rest) {
	case 1:
		head, err := r.Refs.Head()
		if err != nil {
			return err
		}
		if head.Detached() {
			return fmt.Errorf("HEAD is detached, so there is no current branch to rename")
		}
		return porcelain.RenameBranch(r, head.Branch(), rest[0])
	case 2:
		return porcelain.RenameBranch(r, rest[0], rest[1])
	}
	return usagef("branch -m takes one or two names")
}

func (c *Context) listBranches(r *repo.Repo, verbose bool) error {
	list, err := porcelain.ListBranches(r)
	if err != nil {
		return err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return err
	}
	if head.Detached() {
		short, err := r.ODB.Abbrev(head.ID, 7)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "%s\n", c.Style.Green("* (HEAD detached at "+short+")"))
	}
	for _, b := range list {
		line := b.Name
		if b.Current {
			line = c.Style.Green("* " + b.Name)
		} else {
			line = "  " + line
		}
		if verbose {
			short, err := r.ODB.Abbrev(b.ID, 7)
			if err != nil {
				return err
			}
			line += " " + short + " " + b.Subject
		}
		fmt.Fprintln(c.Stdout, line)
	}
	return nil
}

func cmdSwitch(c *Context, args []string) error {
	fs := newFlags("switch")
	create := fs.String("c", "", "create a branch and switch to it")
	detach := fs.Bool("detach", false, "detach HEAD at a commit")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	opts := porcelain.SwitchOptions{Detach: *detach}
	target := ""
	switch {
	case *create != "" && len(rest) <= 1:
		opts.Create, target = true, *create
		if len(rest) == 1 {
			opts.StartPoint = rest[0]
		}
	case *create == "" && len(rest) == 1:
		target = rest[0]
	default:
		return usagef("switch needs a branch name, or -c <name>")
	}
	return c.doSwitch(target, opts)
}

func cmdCheckout(c *Context, args []string) error {
	fs := newFlags("checkout")
	create := fs.String("b", "", "create a branch and switch to it")
	detach := fs.Bool("detach", false, "detach HEAD at a commit")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	opts := porcelain.SwitchOptions{Detach: *detach, Lenient: true}
	target := ""
	switch {
	case *create != "" && len(rest) <= 1:
		opts.Create, target = true, *create
		if len(rest) == 1 {
			opts.StartPoint = rest[0]
		}
	case *create == "" && len(rest) == 1:
		target = rest[0]
	default:
		return usagef("checkout needs a branch or a commit, or -b <name>")
	}
	return c.doSwitch(target, opts)
}

func (c *Context) doSwitch(target string, opts porcelain.SwitchOptions) error {
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	res, err := porcelain.Switch(r, target, opts)
	if err != nil {
		return err
	}
	switch {
	case res.AlreadyOn:
		fmt.Fprintf(c.Stdout, "Already on '%s'\n", res.ToBranch)
	case res.Created:
		fmt.Fprintf(c.Stdout, "Switched to a new branch '%s'\n", res.ToBranch)
	case res.Detached:
		short, err := r.ODB.Abbrev(res.ToID, 7)
		if err != nil {
			return err
		}
		commit, err := r.ODB.ReadCommit(res.ToID)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "Note: switching to '%s'.\n\n", target)
		fmt.Fprintln(c.Stdout, "You are in 'detached HEAD' state. You can look around and make commits,")
		fmt.Fprintln(c.Stdout, "but they will not belong to any branch. To keep them, create a branch:")
		fmt.Fprintf(c.Stdout, "\n  switch -c <new-branch-name>\n\nHEAD is now at %s %s\n", short, commit.Subject())
	default:
		fmt.Fprintf(c.Stdout, "Switched to branch '%s'\n", res.ToBranch)
	}
	return nil
}

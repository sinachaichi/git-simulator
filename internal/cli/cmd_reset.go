package cli

import (
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

func cmdReset(c *Context, args []string) error {
	var dashPaths []string
	hasDashes := false
	for i, a := range args {
		if a == "--" {
			args, dashPaths, hasDashes = args[:i], args[i+1:], true
			break
		}
	}
	fs := newFlags("reset")
	soft := fs.Bool("soft", false, "only move the branch")
	mixed := fs.Bool("mixed", false, "move the branch and reset the index (default)")
	hard := fs.Bool("hard", false, "also reset the working tree")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	modes := 0
	for _, on := range []bool{*soft, *mixed, *hard} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return usagef("use only one of --soft, --mixed and --hard")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}

	// "reset a.txt" and "reset HEAD~1" look alike. The first argument is a
	// revision when it resolves as one and no "--" was given.
	rev := ""
	if len(rest) > 0 {
		if _, err := revparse.Resolve(r, rest[0]); err == nil || hasDashes {
			rev, rest = rest[0], rest[1:]
		}
	}
	paths := append(append([]string(nil), rest...), dashPaths...)
	if len(paths) > 0 {
		if modes > 0 {
			return usagef("a mode cannot be combined with paths")
		}
		specs := make([]string, len(paths))
		for i, p := range paths {
			if specs[i], err = c.repoPath(r, p); err != nil {
				return err
			}
		}
		changed, err := porcelain.ResetPaths(r, rev, specs)
		if err != nil {
			return err
		}
		if len(changed) > 0 {
			fmt.Fprintln(c.Stdout, "Unstaged changes after reset:")
			for _, p := range changed {
				fmt.Fprintf(c.Stdout, "M\t%s\n", p)
			}
		}
		return nil
	}

	mode := porcelain.ResetMixed
	switch {
	case *soft:
		mode = porcelain.ResetSoft
	case *hard:
		mode = porcelain.ResetHard
	}
	if rev == "" {
		rev = "HEAD"
	}
	res, err := porcelain.Reset(r, rev, mode)
	if err != nil {
		return err
	}
	if mode == porcelain.ResetHard {
		commit, err := r.ODB.ReadCommit(res.To)
		if err != nil {
			return err
		}
		short, err := r.ODB.Abbrev(res.To, 7)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "HEAD is now at %s %s\n", short, strings.TrimSpace(commit.Subject()))
	}
	return nil
}

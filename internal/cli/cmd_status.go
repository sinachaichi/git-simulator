package cli

import (
	"fmt"
	"io"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/term"
)

func cmdStatus(c *Context, args []string) error {
	fs := newFlags("status")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return usagef("status takes no arguments")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	res, err := porcelain.Status(r)
	if err != nil {
		return err
	}
	if res.Detached {
		short, err := r.ODB.Abbrev(res.HeadID, 7)
		if err != nil {
			return err
		}
		fmt.Fprintf(c.Stdout, "HEAD detached at %s\n", short)
	}
	writeStatus(c.Stdout, c.Style, res)
	return nil
}

// writeStatus prints the status body in git's layout.
func writeStatus(w io.Writer, s term.Style, res porcelain.StatusResult) {
	if !res.Detached {
		fmt.Fprintf(w, "On branch %s\n", res.Branch)
	}
	if res.Unborn {
		fmt.Fprintf(w, "\nNo commits yet\n")
	}
	if len(res.Unmerged) > 0 {
		fmt.Fprintln(w, "\nUnmerged paths:")
		fmt.Fprintln(w, "  (fix conflicts and run \"add <file>...\" to mark resolution)")
		fmt.Fprintln(w)
		for _, u := range res.Unmerged {
			fmt.Fprintf(w, "\t%s\n", s.Red(fmt.Sprintf("%-17s%s", string(u.Kind)+":", u.Path)))
		}
	}
	if len(res.Staged) > 0 {
		fmt.Fprintln(w, "\nChanges to be committed:")
		if res.Unborn {
			fmt.Fprintln(w, "  (use \"rm --cached <file>...\" to unstage)")
		} else {
			fmt.Fprintln(w, "  (use \"reset HEAD <file>...\" to unstage)")
		}
		fmt.Fprintln(w)
		for _, ch := range res.Staged {
			fmt.Fprintf(w, "\t%s\n", s.Green(fmt.Sprintf("%-12s%s", string(ch.Kind)+":", ch.Path)))
		}
	}
	if len(res.Unstaged) > 0 {
		fmt.Fprintln(w, "\nChanges not staged for commit:")
		fmt.Fprintln(w, "  (use \"add <file>...\" to update what will be committed)")
		fmt.Fprintln(w)
		for _, ch := range res.Unstaged {
			fmt.Fprintf(w, "\t%s\n", s.Red(fmt.Sprintf("%-12s%s", string(ch.Kind)+":", ch.Path)))
		}
	}
	if len(res.Untracked) > 0 {
		fmt.Fprintln(w, "\nUntracked files:")
		fmt.Fprintln(w, "  (use \"add <file>...\" to include in what will be committed)")
		fmt.Fprintln(w)
		for _, p := range res.Untracked {
			fmt.Fprintf(w, "\t%s\n", s.Red(p))
		}
	}
	switch {
	case res.Clean() && res.Unborn:
		fmt.Fprintln(w, "\nnothing to commit (create/copy files and use \"add\" to track)")
	case res.Clean():
		fmt.Fprintln(w, "nothing to commit, working tree clean")
	case len(res.Staged) == 0 && len(res.Unmerged) == 0 && len(res.Unstaged) > 0:
		fmt.Fprintln(w, "\nno changes added to commit (use \"add\" and/or \"commit -a\")")
	case len(res.Staged) == 0 && len(res.Unmerged) == 0:
		fmt.Fprintln(w, "\nnothing added to commit but untracked files present (use \"add\" to track)")
	}
}

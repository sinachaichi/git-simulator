package cli

import (
	"errors"
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
)

// errConflicts makes Run exit with 1 without printing an error line,
// because the conflicts were already listed on stdout.
var errConflicts = errors.New("merge stopped with conflicts")

func cmdMerge(c *Context, args []string) error {
	fs := newFlags("merge")
	noFF := fs.Bool("no-ff", false, "always create a merge commit")
	ffOnly := fs.Bool("ff-only", false, "fail unless the merge is a fast-forward")
	noCommit := fs.Bool("no-commit", false, "stop before creating the merge commit")
	abort := fs.Bool("abort", false, "give up the merge in progress")
	var messages stringList
	fs.Var(&messages, "m", "message of the merge commit")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	if *abort {
		if len(rest) > 0 {
			return usagef("merge --abort takes no revision")
		}
		return porcelain.MergeAbort(r)
	}
	if len(rest) != 1 {
		return usagef("merge needs one branch or commit")
	}
	if *noFF && *ffOnly {
		return usagef("--no-ff and --ff-only cannot be used together")
	}
	res, err := porcelain.Merge(r, rest[0], porcelain.MergeOptions{
		NoFF: *noFF, FFOnly: *ffOnly, NoCommit: *noCommit, Message: joinMessages(messages),
	})
	if err != nil {
		return err
	}
	short := func(id fmt.Stringer) string {
		s := id.String()
		return s[:7]
	}
	switch res.Outcome {
	case porcelain.MergeUpToDate:
		fmt.Fprintln(c.Stdout, "Already up to date.")
	case porcelain.MergeFastForward:
		fmt.Fprintf(c.Stdout, "Updating %s..%s\nFast-forward\n", short(res.From), short(res.Commit))
	case porcelain.MergeCommitted:
		fmt.Fprintln(c.Stdout, "Merge made by the 'ort' strategy.")
	case porcelain.MergeStopped:
		fmt.Fprintln(c.Stdout, "Automatic merge went well; stopped before committing as requested")
	case porcelain.MergeConflicted:
		for _, cf := range res.Conflicts {
			fmt.Fprintln(c.Stdout, c.conflictLine(cf, rest[0]))
		}
		fmt.Fprintln(c.Stdout, "Automatic merge failed; fix conflicts and then commit the result.")
		return errConflicts
	}
	return nil
}

// conflictLine words a conflict the way git does.
func (c *Context) conflictLine(cf porcelain.ConflictInfo, theirs string) string {
	switch cf.Kind {
	case merge.ModifyDelete:
		deleted, kept := "HEAD", theirs
		if !cf.HasTheirs {
			deleted, kept = theirs, "HEAD"
		}
		return fmt.Sprintf("CONFLICT (modify/delete): %s deleted in %s and modified in %s. Version %s of %s left in tree.",
			cf.Path, deleted, kept, kept, cf.Path)
	case merge.AddAdd:
		return fmt.Sprintf("CONFLICT (add/add): Merge conflict in %s", cf.Path)
	case merge.BinaryConflict:
		return fmt.Sprintf("CONFLICT (content): Merge conflict in %s (binary file)", cf.Path)
	case merge.ModeConflict:
		return fmt.Sprintf("CONFLICT (mode): Merge conflict in %s", cf.Path)
	}
	return fmt.Sprintf("CONFLICT (content): Merge conflict in %s", cf.Path)
}

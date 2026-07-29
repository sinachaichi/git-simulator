package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func cmdInit(c *Context, args []string) error {
	fs := newFlags("init")
	branch := fs.String("b", repo.DefaultBranch, "name of the first branch")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return usagef("init takes at most one directory")
	}
	dir := c.Cwd
	if len(rest) == 1 {
		dir = rest[0]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(c.Cwd, dir)
		}
	}
	r, err := repo.Init(dir, *branch)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "Initialized empty hood repository in %s/\n", r.GitDir)
	return nil
}

func cmdAdd(c *Context, args []string) error {
	fs := newFlags("add")
	all := fs.Bool("A", false, "stage everything")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *all {
		rest = append(rest, ".")
	}
	if len(rest) == 0 {
		return usagef("nothing specified. Say which files to add, or use `add .`")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	specs := make([]string, len(rest))
	for i, p := range rest {
		if specs[i], err = c.repoPath(r, p); err != nil {
			return err
		}
	}
	_, err = porcelain.Add(r, specs)
	return err
}

func cmdCommit(c *Context, args []string) error {
	fs := newFlags("commit")
	var messages stringList
	fs.Var(&messages, "m", "commit message, can be repeated")
	all := fs.Bool("a", false, "stage changes to tracked files first")
	amend := fs.Bool("amend", false, "replace the tip commit")
	empty := fs.Bool("allow-empty", false, "allow a commit with no changes")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return usagef("unexpected argument %q", rest[0])
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	res, err := porcelain.Commit(r, porcelain.CommitOptions{
		Message: joinMessages(messages), All: *all, Amend: *amend, AllowEmpty: *empty,
	})
	if err != nil {
		return err
	}
	where := res.Branch
	switch {
	case res.Branch == "":
		where = "detached HEAD"
	case res.Root:
		where += " (root-commit)"
	}
	short, err := r.ODB.Abbrev(res.ID, 7)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "[%s %s] %s\n", where, short, res.Commit.Subject())
	return nil
}

// joinMessages turns repeated -m values into paragraphs.
func joinMessages(messages []string) string {
	return strings.Join(messages, "\n\n")
}

func cmdConfig(c *Context, args []string) error {
	if len(args) == 0 || len(args) > 2 {
		return usagef("config needs a key and an optional value")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	if len(args) == 2 {
		return r.ConfigSet(args[0], args[1])
	}
	v, ok, err := r.ConfigGet(args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s is not set", args[0])
	}
	fmt.Fprintln(c.Stdout, v)
	return nil
}

func cmdCatFile(c *Context, args []string) error {
	fs := newFlags("cat-file")
	typ := fs.Bool("t", false, "show the object type")
	size := fs.Bool("s", false, "show the object size")
	pretty := fs.Bool("p", false, "show the content")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return usagef("cat-file needs one object")
	}
	mode := porcelain.CatFileMode("")
	switch {
	case *typ && !*size && !*pretty:
		mode = porcelain.CatType
	case *size && !*typ && !*pretty:
		mode = porcelain.CatSize
	case *pretty && !*typ && !*size:
		mode = porcelain.CatPretty
	default:
		return usagef("use exactly one of -t, -s or -p")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	out, err := porcelain.CatFile(r, rest[0], mode)
	if err != nil {
		return err
	}
	_, err = io.WriteString(c.Stdout, out)
	return err
}

func cmdHashObject(c *Context, args []string) error {
	fs := newFlags("hash-object")
	write := fs.Bool("w", false, "store the object")
	stdin := fs.Bool("stdin", false, "read from standard input")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	var data []byte
	switch {
	case *stdin && len(rest) == 0:
		data, err = io.ReadAll(c.Stdin)
	case !*stdin && len(rest) == 1:
		p := rest[0]
		if !filepath.IsAbs(p) {
			p = filepath.Join(c.Cwd, p)
		}
		data, err = os.ReadFile(p)
	default:
		return usagef("give one file, or --stdin")
	}
	if err != nil {
		return err
	}
	r, err := c.openRepo()
	if err != nil {
		if *write {
			return err
		}
		r = nil
	}
	id, err := porcelain.HashObject(r, data, *write)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, id)
	return nil
}

func cmdLsFiles(c *Context, args []string) error {
	fs := newFlags("ls-files")
	stage := fs.Bool("stage", false, "show mode, id and stage")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return usagef("ls-files takes no paths yet")
	}
	r, err := c.openRepo()
	if err != nil {
		return err
	}
	lines, err := porcelain.LsFiles(r, *stage)
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(c.Stdout, l)
	}
	return nil
}

package porcelain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// Errors returned by Commit.
var (
	ErrNothingToCommit = errors.New("nothing to commit")
	ErrEmptyMessage    = errors.New("empty commit message")
)

// CommitOptions are the flags of `commit`.
type CommitOptions struct {
	Message    string
	All        bool // -a: stage changes to tracked files first
	Amend      bool // --amend: replace the tip commit
	AllowEmpty bool // --allow-empty
}

// CommitResult describes the new commit.
type CommitResult struct {
	ID      objects.ID
	Commit  *objects.Commit
	Branch  string // empty when HEAD was detached
	Root    bool   // the commit has no parent
	Amended bool
}

// Commit records the index as a new commit on HEAD.
func Commit(r *repo.Repo, opts CommitOptions) (CommitResult, error) {
	ix, err := r.LoadIndex()
	if err != nil {
		return CommitResult{}, err
	}
	if opts.All {
		if err := stageTracked(r, ix); err != nil {
			return CommitResult{}, err
		}
		if err := r.SaveIndex(ix); err != nil {
			return CommitResult{}, err
		}
	}
	treeID, err := ix.WriteTree(r.ODB)
	if err != nil {
		return CommitResult{}, err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return CommitResult{}, err
	}

	var parents []objects.ID
	var old *objects.Commit
	if !head.Unborn() {
		if old, err = r.ODB.ReadCommit(head.ID); err != nil {
			return CommitResult{}, err
		}
		parents = []objects.ID{head.ID}
	}
	if opts.Amend {
		if old == nil {
			return CommitResult{}, errors.New("cannot amend: there is no commit yet")
		}
		parents = old.Parents
	}
	if !opts.AllowEmpty && !opts.Amend && old != nil && old.Tree == treeID {
		return CommitResult{}, ErrNothingToCommit
	}
	if old == nil && ix.Len() == 0 && !opts.AllowEmpty {
		return CommitResult{}, ErrNothingToCommit
	}

	message := CleanMessage(opts.Message)
	if message == "" && opts.Amend {
		message = old.Message
	}
	if message == "" {
		return CommitResult{}, ErrEmptyMessage
	}
	committer, err := r.Identity("committer")
	if err != nil {
		return CommitResult{}, err
	}
	var author objects.Signature
	if opts.Amend {
		author = old.Author
	} else if author, err = r.Identity("author"); err != nil {
		return CommitResult{}, err
	}

	c := &objects.Commit{Tree: treeID, Parents: parents, Author: author, Committer: committer, Message: message}
	id, _, err := r.ODB.Write(c)
	if err != nil {
		return CommitResult{}, err
	}
	kind := "commit"
	switch {
	case opts.Amend:
		kind = "commit (amend)"
	case len(parents) == 0:
		kind = "commit (initial)"
	}
	if err := moveHead(r, id, kind+": "+c.Subject()); err != nil {
		return CommitResult{}, err
	}
	return CommitResult{ID: id, Commit: c, Branch: head.Branch(), Root: len(parents) == 0, Amended: opts.Amend}, nil
}

// stageTracked re-stages every tracked file, like `commit -a`. New files
// are left alone.
func stageTracked(r *repo.Repo, ix *index.Index) error {
	for _, p := range ix.Paths() {
		if _, ok := ix.Get(p); !ok {
			continue // conflicted paths must be resolved with add
		}
		entry, err := stageFile(r, p)
		switch {
		case isGone(err):
			ix.Remove(p)
		case err != nil:
			return fmt.Errorf("staging %s: %w", p, err)
		default:
			ix.Add(entry)
		}
	}
	return nil
}

// CleanMessage applies git's default cleanup: trailing spaces are cut,
// blank lines at the start and end are dropped, runs of blank lines
// shrink to one, and the message ends with a single newline. An empty
// result stays empty.
func CleanMessage(m string) string {
	var lines []string
	blank := false
	for _, line := range strings.Split(strings.ReplaceAll(m, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			blank = len(lines) > 0
			continue
		}
		if blank {
			lines = append(lines, "")
			blank = false
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

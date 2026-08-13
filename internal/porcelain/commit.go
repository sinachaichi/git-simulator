package porcelain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

	reflogMessage string // set by Merge to word the reflog like git does
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

	mergeParent, mergeMessage, merging, err := readMergeState(r)
	if err != nil {
		return CommitResult{}, err
	}
	pickID, _, picking, err := readPickState(r)
	if err != nil {
		return CommitResult{}, err
	}
	if (merging || picking) && opts.Amend {
		return CommitResult{}, errors.New("cannot amend while a merge, cherry-pick or revert is in progress")
	}
	if picking && mergeMessage == "" {
		data, err := os.ReadFile(filepath.Join(r.GitDir, mergeMsgFile))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return CommitResult{}, err
		}
		mergeMessage = string(data)
	}

	var parents []objects.ID
	var old *objects.Commit
	if !head.Unborn() {
		if old, err = r.ODB.ReadCommit(head.ID); err != nil {
			return CommitResult{}, err
		}
		parents = []objects.ID{head.ID}
	}
	if merging {
		parents = append(parents, mergeParent)
	}
	if opts.Amend {
		if old == nil {
			return CommitResult{}, errors.New("cannot amend: there is no commit yet")
		}
		parents = old.Parents
	}
	if !opts.AllowEmpty && !opts.Amend && !merging && old != nil && old.Tree == treeID {
		return CommitResult{}, ErrNothingToCommit
	}
	if old == nil && ix.Len() == 0 && !opts.AllowEmpty {
		return CommitResult{}, ErrNothingToCommit
	}

	message := CleanMessage(opts.Message)
	if message == "" && opts.Amend {
		message = old.Message
	}
	if message == "" && (merging || picking) {
		message = mergeMessage
	}
	if message == "" {
		return CommitResult{}, ErrEmptyMessage
	}
	committer, err := r.Identity("committer")
	if err != nil {
		return CommitResult{}, err
	}
	var author objects.Signature
	switch {
	case opts.Amend:
		author = old.Author
	case picking && !isRevert(r):
		// A cherry-picked commit keeps its author.
		picked, err := r.ODB.ReadCommit(pickID)
		if err != nil {
			return CommitResult{}, err
		}
		author = picked.Author
	default:
		if author, err = r.Identity("author"); err != nil {
			return CommitResult{}, err
		}
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
	case merging:
		kind = "commit (merge)"
	case len(parents) == 0:
		kind = "commit (initial)"
	}
	logMessage := kind + ": " + c.Subject()
	if opts.reflogMessage != "" {
		logMessage = opts.reflogMessage
	}
	if err := moveHead(r, id, logMessage); err != nil {
		return CommitResult{}, err
	}
	if merging || picking {
		clearMergeState(r)
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

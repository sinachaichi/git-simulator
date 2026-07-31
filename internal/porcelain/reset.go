package porcelain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// ResetMode says how far a reset reaches.
type ResetMode string

// The three reset modes.
const (
	ResetSoft  ResetMode = "soft"  // move the branch only
	ResetMixed ResetMode = "mixed" // also reset the index
	ResetHard  ResetMode = "hard"  // also reset the working tree
)

// ResetResult says where HEAD was and where it is now.
type ResetResult struct {
	From objects.ID
	To   objects.ID
	Mode ResetMode
}

// origHeadFile holds the previous HEAD after a reset, as in git.
const origHeadFile = "ORIG_HEAD"

// Reset moves the current branch (or detached HEAD) to rev. The old
// position is saved in ORIG_HEAD so it is easy to go back.
func Reset(r *repo.Repo, rev string, mode ResetMode) (ResetResult, error) {
	head, err := r.Refs.Head()
	if err != nil {
		return ResetResult{}, err
	}
	target, err := revparse.ResolveCommit(r, rev)
	if err != nil {
		return ResetResult{}, err
	}
	res := ResetResult{From: head.ID, To: target, Mode: mode}
	switch mode {
	case ResetSoft, ResetMixed, ResetHard:
	default:
		return res, fmt.Errorf("unknown reset mode %q", mode)
	}
	if mode == ResetSoft {
		if ix, err := r.LoadIndex(); err != nil {
			return res, err
		} else if len(ix.Conflicts()) > 0 {
			return res, fmt.Errorf("cannot do a soft reset in the middle of a merge")
		}
	}

	if !head.Unborn() {
		if err := writeState(r, origHeadFile, head.ID.String()+"\n"); err != nil {
			return res, err
		}
	}
	if mode != ResetSoft {
		if err := resetIndexAndFiles(r, target, mode == ResetHard); err != nil {
			return res, err
		}
	}
	if err := moveHead(r, target, "reset: moving to "+rev); err != nil {
		return res, err
	}
	if mode != ResetSoft {
		clearMergeState(r)
	}
	return res, nil
}

// resetIndexAndFiles makes the index equal to the target tree, and the
// working tree too when hard is set. Untracked files are left alone.
func resetIndexAndFiles(r *repo.Repo, target objects.ID, hard bool) error {
	tgt, err := commitFiles(r, target)
	if err != nil {
		return err
	}
	ix, err := r.LoadIndex()
	if err != nil {
		return err
	}
	if hard {
		// Every tracked path is rewritten, so local edits are discarded.
		touched := map[string]bool{}
		for _, p := range ix.Paths() {
			touched[p] = true
		}
		cur, err := headFiles(r)
		if err != nil {
			return err
		}
		for p := range cur {
			touched[p] = true
		}
		for p := range tgt {
			touched[p] = true
		}
		paths := make([]string, 0, len(touched))
		for p := range touched {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		if err := syncPaths(r, ix, tgt, paths); err != nil {
			return err
		}
	} else {
		ix = index.New()
		files := make([]tree.File, 0, len(tgt))
		for _, f := range tgt {
			files = append(files, f)
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		for _, f := range files {
			ix.Add(index.Entry{Path: f.Path, Mode: f.Mode, ID: f.ID})
		}
	}
	return r.SaveIndex(ix)
}

// clearMergeState forgets an unfinished merge. A reset ends it, which is
// also how `merge --abort` is done.
func clearMergeState(r *repo.Repo) {
	for _, name := range []string{"MERGE_HEAD", "MERGE_MSG"} {
		_ = os.Remove(filepath.Join(r.GitDir, name))
	}
}

func writeState(r *repo.Repo, name, content string) error {
	return os.WriteFile(filepath.Join(r.GitDir, name), []byte(content), 0o644)
}

package porcelain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// Errors returned by the branch functions.
var (
	ErrBranchExists   = errors.New("branch already exists")
	ErrBranchNotFound = errors.New("branch not found")
	ErrCurrentBranch  = errors.New("cannot change the branch you are on")
	ErrNotMerged      = errors.New("branch is not fully merged")
)

// BranchInfo is one line of the branch list.
type BranchInfo struct {
	Name    string
	ID      objects.ID
	Subject string
	Current bool
}

// ListBranches returns all branches sorted by name.
func ListBranches(r *repo.Repo) ([]BranchInfo, error) {
	list, err := r.Refs.List(refs.BranchPrefix)
	if err != nil {
		return nil, err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return nil, err
	}
	out := make([]BranchInfo, 0, len(list))
	for _, b := range list {
		c, err := r.ODB.ReadCommit(b.ID)
		if err != nil {
			return nil, fmt.Errorf("branch %s: %w", b.Name[len(refs.BranchPrefix):], err)
		}
		out = append(out, BranchInfo{
			Name: strings.TrimPrefix(b.Name, refs.BranchPrefix), ID: b.ID,
			Subject: c.Subject(), Current: b.Name == head.Ref,
		})
	}
	return out, nil
}

// CreateBranch makes a new branch at start (HEAD when empty).
func CreateBranch(r *repo.Repo, name, start string) (objects.ID, error) {
	ref := refs.BranchPrefix + name
	if err := refs.ValidName(ref); err != nil {
		return objects.Zero, err
	}
	if r.Refs.Exists(ref) {
		return objects.Zero, fmt.Errorf("%w: %s", ErrBranchExists, name)
	}
	if start == "" {
		start = "HEAD"
	}
	id, err := revparse.ResolveCommit(r, start)
	if err != nil {
		return objects.Zero, err
	}
	return id, updateRef(r, ref, id, "branch: Created from "+start)
}

// DeleteBranch removes a branch. Unless force is set, the branch must be
// merged into HEAD, so no commit becomes unreachable by accident.
func DeleteBranch(r *repo.Repo, name string, force bool) (objects.ID, error) {
	ref := refs.BranchPrefix + name
	id, err := r.Refs.Read(ref)
	if errors.Is(err, refs.ErrNotFound) {
		return objects.Zero, fmt.Errorf("%w: %s", ErrBranchNotFound, name)
	}
	if err != nil {
		return objects.Zero, err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return objects.Zero, err
	}
	if head.Ref == ref {
		return objects.Zero, fmt.Errorf("%w: %s", ErrCurrentBranch, name)
	}
	if !force {
		merged := false
		if !head.Unborn() {
			if merged, err = history.IsAncestor(r.ODB, id, head.ID); err != nil {
				return objects.Zero, err
			}
		}
		if !merged {
			return objects.Zero, fmt.Errorf("%w: %s", ErrNotMerged, name)
		}
	}
	if err := r.Refs.Delete(ref); err != nil {
		return objects.Zero, err
	}
	return id, r.Refs.DeleteLog(ref)
}

// RenameBranch renames a branch, together with its reflog. HEAD follows
// when the branch is the current one.
func RenameBranch(r *repo.Repo, oldName, newName string) error {
	oldRef, newRef := refs.BranchPrefix+oldName, refs.BranchPrefix+newName
	if err := refs.ValidName(newRef); err != nil {
		return err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return err
	}
	id, err := r.Refs.Read(oldRef)
	if errors.Is(err, refs.ErrNotFound) && head.Ref == oldRef && head.Unborn() {
		return r.Refs.SetHeadBranch(newName) // renaming an unborn branch
	}
	if errors.Is(err, refs.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrBranchNotFound, oldName)
	}
	if err != nil {
		return err
	}
	if r.Refs.Exists(newRef) {
		return fmt.Errorf("%w: %s", ErrBranchExists, newName)
	}
	if err := r.Refs.Update(newRef, id); err != nil {
		return err
	}
	if err := r.Refs.MoveLog(oldRef, newRef); err != nil {
		return err
	}
	if err := r.Refs.Delete(oldRef); err != nil {
		return err
	}
	if head.Ref == oldRef {
		return r.Refs.SetHeadBranch(newName)
	}
	return nil
}

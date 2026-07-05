package porcelain

import (
	"errors"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// updateRef moves a ref and records the move in its reflog. When the ref
// is the branch HEAD is on, HEAD's own reflog gets the entry too, which
// is how git behaves.
func updateRef(r *repo.Repo, name string, id objects.ID, msg string) error {
	old, err := r.Refs.Read(name)
	if err != nil && !errors.Is(err, refs.ErrNotFound) {
		return err
	}
	if err := r.Refs.Update(name, id); err != nil {
		return err
	}
	who, err := r.Identity("committer")
	if err != nil {
		return err
	}
	entry := refs.LogEntry{Old: old, New: id, Who: who, Message: msg}
	if err := r.Refs.AppendLog(name, entry); err != nil {
		return err
	}
	head, err := r.Refs.Head()
	if err != nil {
		return err
	}
	if head.Ref == name {
		return r.Refs.AppendLog("HEAD", entry)
	}
	return nil
}

// moveHead points the current branch, or a detached HEAD, at id and logs it.
func moveHead(r *repo.Repo, id objects.ID, msg string) error {
	head, err := r.Refs.Head()
	if err != nil {
		return err
	}
	if !head.Detached() {
		return updateRef(r, head.Ref, id, msg)
	}
	who, err := r.Identity("committer")
	if err != nil {
		return err
	}
	if err := r.Refs.SetHeadDetached(id); err != nil {
		return err
	}
	return r.Refs.AppendLog("HEAD", refs.LogEntry{Old: head.ID, New: id, Who: who, Message: msg})
}

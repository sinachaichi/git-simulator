package porcelain

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// ReflogLine is one entry of `reflog`.
type ReflogLine struct {
	Selector string // "HEAD@{0}"
	ID       objects.ID
	Message  string
}

// Reflog returns the log of a ref, newest first. ref is HEAD, a branch
// name or "stash". An unknown ref is an error, an empty log is not.
func Reflog(r *repo.Repo, ref string) ([]ReflogLine, error) {
	if ref == "" {
		ref = "HEAD"
	}
	name := ref
	switch {
	case ref == "HEAD":
	case ref == "stash":
		name = stashRef
	case r.Refs.Exists(refs.BranchPrefix + ref):
		name = refs.BranchPrefix + ref
	default:
		return nil, fmt.Errorf("%w: %s", ErrBranchNotFound, ref)
	}
	entries, err := r.Refs.ReadLog(name)
	if err != nil {
		return nil, err
	}
	out := make([]ReflogLine, len(entries))
	for i, e := range entries {
		out[i] = ReflogLine{Selector: fmt.Sprintf("%s@{%d}", ref, i), ID: e.New, Message: e.Message}
	}
	return out, nil
}

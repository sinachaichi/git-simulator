package porcelain

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// ShowResult is what `show` prints for one object. Only the fields that
// apply to the object type are set.
type ShowResult struct {
	ID     objects.ID
	Tag    *objects.Tag    // annotated tag header, before the object it points at
	Commit *objects.Commit // commit header and message
	Files  []diff.File     // changes made by the commit, against its first parent
	Tree   *objects.Tree   // listing of a tree
	Blob   *objects.Blob   // content of a blob
	Next   *ShowResult     // the object an annotated tag points to
}

// Show resolves rev and collects what to print. Commits come with their
// patch against the first parent, or against nothing for a root commit.
// Merge commits get no patch, since git would print a combined diff.
func Show(r *repo.Repo, rev string) (ShowResult, error) {
	id, err := revparse.Resolve(r, rev)
	if err != nil {
		return ShowResult{}, err
	}
	return showObject(r, id)
}

func showObject(r *repo.Repo, id objects.ID) (ShowResult, error) {
	o, err := r.ODB.Read(id)
	if err != nil {
		return ShowResult{}, err
	}
	res := ShowResult{ID: id}
	switch v := o.(type) {
	case *objects.Blob:
		res.Blob = v
	case *objects.Tree:
		res.Tree = v
	case *objects.Tag:
		res.Tag = v
		next, err := showObject(r, v.Object)
		if err != nil {
			return res, err
		}
		res.Next = &next
	case *objects.Commit:
		res.Commit = v
		if len(v.Parents) > 1 {
			return res, nil
		}
		oldSide := side{} // a root commit is compared with nothing
		if len(v.Parents) == 1 {
			parent, err := r.ODB.ReadCommit(v.Parents[0])
			if err != nil {
				return res, err
			}
			if oldSide, err = treeSide(r, parent.Tree); err != nil {
				return res, err
			}
		}
		newSide, err := treeSide(r, v.Tree)
		if err != nil {
			return res, err
		}
		if res.Files, err = compare(oldSide, newSide, nil); err != nil {
			return res, err
		}
	default:
		return res, fmt.Errorf("cannot show a %s object", o.Type())
	}
	return res, nil
}

func treeSide(r *repo.Repo, id objects.ID) (side, error) {
	files, err := tree.Flatten(r.ODB, id)
	if err != nil {
		return nil, err
	}
	s := side{}
	for _, f := range files {
		s[f.Path] = sideFile{mode: f.Mode, id: f.ID, read: blobReader(r, f.ID)}
	}
	return s, nil
}

package porcelain

import (
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// headFiles returns the files of the HEAD commit by path. An unborn
// branch has none.
func headFiles(r *repo.Repo) (map[string]tree.File, error) {
	head, err := r.Refs.Head()
	if err != nil {
		return nil, err
	}
	if head.Unborn() {
		return map[string]tree.File{}, nil
	}
	return commitFiles(r, head.ID)
}

// commitFiles returns the files of a commit by path.
func commitFiles(r *repo.Repo, id objects.ID) (map[string]tree.File, error) {
	c, err := r.ODB.ReadCommit(id)
	if err != nil {
		return nil, err
	}
	files, err := tree.Flatten(r.ODB, c.Tree)
	if err != nil {
		return nil, err
	}
	out := make(map[string]tree.File, len(files))
	for _, f := range files {
		out[f.Path] = f
	}
	return out, nil
}

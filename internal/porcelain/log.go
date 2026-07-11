package porcelain

import (
	"fmt"
	"sort"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// LogOptions are the flags of `log`.
type LogOptions struct {
	Revs []string // starting points, default HEAD
	All  bool     // --all: start from every branch, tag and HEAD
	Max  int      // -n, 0 means no limit
}

// Log returns commits newest first.
func Log(r *repo.Repo, opts LogOptions) ([]history.Node, error) {
	var starts []objects.ID
	if opts.All {
		ids, err := allTips(r)
		if err != nil {
			return nil, err
		}
		starts = append(starts, ids...)
	}
	revs := opts.Revs
	if len(revs) == 0 && !opts.All {
		revs = []string{"HEAD"}
	}
	for _, rev := range revs {
		id, err := revparse.ResolveCommit(r, rev)
		if err != nil {
			return nil, err
		}
		starts = append(starts, id)
	}
	nodes, err := history.Walk(r.ODB, starts)
	if err != nil {
		return nil, err
	}
	if opts.Max > 0 && len(nodes) > opts.Max {
		nodes = nodes[:opts.Max]
	}
	return nodes, nil
}

// allTips returns the commits that HEAD, branches and tags point to.
func allTips(r *repo.Repo) ([]objects.ID, error) {
	var ids []objects.ID
	if head, err := r.Refs.Head(); err != nil {
		return nil, err
	} else if !head.Unborn() {
		ids = append(ids, head.ID)
	}
	for _, prefix := range []string{refs.BranchPrefix, refs.TagPrefix} {
		list, err := r.Refs.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, ref := range list {
			id, err := peelToCommit(r, ref.ID)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ref.Name, err)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// peelToCommit follows annotated tags until it finds a commit.
func peelToCommit(r *repo.Repo, id objects.ID) (objects.ID, error) {
	for {
		o, err := r.ODB.Read(id)
		if err != nil {
			return objects.Zero, err
		}
		switch v := o.(type) {
		case *objects.Commit:
			return id, nil
		case *objects.Tag:
			id = v.Object
		default:
			return objects.Zero, fmt.Errorf("points to a %s, not a commit", o.Type())
		}
	}
}

// Decorations maps commit ids to the ref names shown next to them in
// `log`, such as "HEAD -> main", "tag: v1" or "feature".
func Decorations(r *repo.Repo) (map[objects.ID][]string, error) {
	out := map[objects.ID][]string{}
	head, err := r.Refs.Head()
	if err != nil {
		return nil, err
	}
	if !head.Unborn() {
		if head.Detached() {
			out[head.ID] = append(out[head.ID], "HEAD")
		} else {
			out[head.ID] = append(out[head.ID], "HEAD -> "+head.Branch())
		}
	}
	tags, err := r.Refs.List(refs.TagPrefix)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		id, err := peelToCommit(r, t.ID)
		if err != nil {
			return nil, err
		}
		out[id] = append(out[id], "tag: "+t.Name[len(refs.TagPrefix):])
	}
	branches, err := r.Refs.List(refs.BranchPrefix)
	if err != nil {
		return nil, err
	}
	for _, b := range branches {
		if b.Name == head.Ref {
			continue
		}
		out[b.ID] = append(out[b.ID], b.Name[len(refs.BranchPrefix):])
	}
	for _, names := range out {
		sort.SliceStable(names, func(i, j int) bool { return decorationRank(names[i]) < decorationRank(names[j]) })
	}
	return out, nil
}

func decorationRank(name string) int {
	switch {
	case name == "HEAD" || len(name) > 4 && name[:4] == "HEAD":
		return 0
	case len(name) > 5 && name[:5] == "tag: ":
		return 1
	}
	return 2
}

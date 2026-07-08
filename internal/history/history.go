// Package history walks the commit graph.
package history

import (
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
)

// Node is a commit with its id.
type Node struct {
	ID     objects.ID
	Commit *objects.Commit
}

// Walk returns every commit reachable from starts, newest committer date
// first. Commits with the same date keep the order they were found in,
// so a merge commit stays ahead of its parents.
func Walk(s *odb.Store, starts []objects.ID) ([]Node, error) {
	seen := map[objects.ID]bool{}
	var queue, out []Node
	push := func(id objects.ID) error {
		if seen[id] {
			return nil
		}
		seen[id] = true
		c, err := s.ReadCommit(id)
		if err != nil {
			return err
		}
		queue = append(queue, Node{ID: id, Commit: c})
		return nil
	}
	for _, id := range starts {
		if err := push(id); err != nil {
			return nil, err
		}
	}
	for len(queue) > 0 {
		best := 0
		for i, n := range queue {
			if n.Commit.Committer.When.After(queue[best].Commit.Committer.When) {
				best = i
			}
		}
		n := queue[best]
		queue = append(queue[:best], queue[best+1:]...)
		out = append(out, n)
		for _, p := range n.Commit.Parents {
			if err := push(p); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

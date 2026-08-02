// Package history walks the commit graph.
package history

import (
	"sort"

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

// IsAncestor reports whether ancestor is reachable from descendant. A
// commit counts as its own ancestor, like in git.
func IsAncestor(s *odb.Store, ancestor, descendant objects.ID) (bool, error) {
	nodes, err := Walk(s, []objects.ID{descendant})
	if err != nil {
		return false, err
	}
	for _, n := range nodes {
		if n.ID == ancestor {
			return true, nil
		}
	}
	return false, nil
}

// MergeBases returns the best common ancestors of a and b: commits that
// both can reach and that are not ancestors of another common commit.
// There is usually one. The result is empty for unrelated histories.
func MergeBases(s *odb.Store, a, b objects.ID) ([]objects.ID, error) {
	reachable := func(id objects.ID) (map[objects.ID]*objects.Commit, error) {
		nodes, err := Walk(s, []objects.ID{id})
		if err != nil {
			return nil, err
		}
		m := make(map[objects.ID]*objects.Commit, len(nodes))
		for _, n := range nodes {
			m[n.ID] = n.Commit
		}
		return m, nil
	}
	fromA, err := reachable(a)
	if err != nil {
		return nil, err
	}
	fromB, err := reachable(b)
	if err != nil {
		return nil, err
	}
	common := map[objects.ID]bool{}
	for id := range fromA {
		if _, ok := fromB[id]; ok {
			common[id] = true
		}
	}
	// A common commit is not "best" when another common commit reaches it.
	hidden := map[objects.ID]bool{}
	for id := range common {
		nodes, err := Walk(s, fromA[id].Parents)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			hidden[n.ID] = true
		}
	}
	var bases []Node
	for id := range common {
		if !hidden[id] {
			bases = append(bases, Node{ID: id, Commit: fromA[id]})
		}
	}
	sort.Slice(bases, func(i, j int) bool {
		bi, bj := bases[i], bases[j]
		if !bi.Commit.Committer.When.Equal(bj.Commit.Committer.When) {
			return bi.Commit.Committer.When.After(bj.Commit.Committer.When)
		}
		return bi.ID.String() < bj.ID.String()
	})
	ids := make([]objects.ID, len(bases))
	for i, n := range bases {
		ids[i] = n.ID
	}
	return ids, nil
}

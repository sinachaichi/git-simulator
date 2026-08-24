package refs

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

const headFile = "HEAD"

// Head describes where HEAD points.
type Head struct {
	// Ref is the full branch ref name, or empty when HEAD is detached.
	Ref string
	// ID is the commit HEAD resolves to. It is Zero on an unborn branch.
	ID objects.ID
}

// Detached reports whether HEAD points straight at a commit.
func (h Head) Detached() bool { return h.Ref == "" }

// Branch returns the short branch name, or "" when HEAD is detached.
func (h Head) Branch() string { return strings.TrimPrefix(h.Ref, BranchPrefix) }

// Unborn reports whether HEAD is on a branch that has no commit yet.
func (h Head) Unborn() bool { return h.ID.IsZero() }

// SetHeadBranch makes HEAD a symbolic ref to refs/heads/<branch>. The
// branch does not have to exist yet, which is how a new repo starts.
func (s *Store) SetHeadBranch(branch string) error {
	name := BranchPrefix + branch
	if err := ValidName(name); err != nil {
		return err
	}
	if err := s.writeFile(headFile, "ref: "+name+"\n"); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.HeadSet, Subject: headFile, New: name, Detail: "symbolic"})
	return nil
}

// SetHeadDetached makes HEAD point straight at a commit.
func (s *Store) SetHeadDetached(id objects.ID) error {
	if err := s.writeFile(headFile, id.String()+"\n"); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.HeadSet, Subject: headFile, New: id.String(), Detail: "detached"})
	return nil
}

// Head reads HEAD and resolves it.
func (s *Store) Head() (Head, error) {
	data, err := os.ReadFile(s.path(headFile))
	if err != nil {
		return Head{}, fmt.Errorf("cannot read HEAD: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if target, ok := strings.CutPrefix(text, "ref: "); ok {
		id, err := s.Read(target)
		if errors.Is(err, ErrNotFound) {
			return Head{Ref: target}, nil
		}
		if err != nil {
			return Head{}, err
		}
		return Head{Ref: target, ID: id}, nil
	}
	id, err := objects.ParseID(text)
	if err != nil {
		return Head{}, fmt.Errorf("HEAD is corrupt: %w", err)
	}
	return Head{ID: id}, nil
}

// MoveHead points the current branch, or HEAD itself when detached, at id.
func (s *Store) MoveHead(id objects.ID) error {
	h, err := s.Head()
	if err != nil {
		return err
	}
	if h.Detached() {
		return s.SetHeadDetached(id)
	}
	return s.Update(h.Ref, id)
}

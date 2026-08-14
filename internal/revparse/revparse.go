// Package revparse turns revision expressions such as "main~2", "HEAD^2"
// or "ab12cd3" into object ids, following the rules of git's rev-parse.
package revparse

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// NotFoundError means the expression does not name anything.
type NotFoundError struct {
	Expr string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("unknown revision %q", e.Expr)
}

// Resolve returns the object an expression points to. The result can be
// any object type, for example an annotated tag stays a tag.
func Resolve(r *repo.Repo, expr string) (objects.ID, error) {
	base, ops := splitOps(expr)
	id, err := resolveBase(r, base, expr)
	if err != nil {
		return objects.Zero, err
	}
	for _, op := range ops {
		if id, err = apply(r, id, op, expr); err != nil {
			return objects.Zero, err
		}
	}
	return id, nil
}

// ResolveCommit is Resolve followed by peeling annotated tags, and fails
// if the result is not a commit.
func ResolveCommit(r *repo.Repo, expr string) (objects.ID, error) {
	id, err := Resolve(r, expr)
	if err != nil {
		return objects.Zero, err
	}
	return peel(r, id, objects.TypeCommit)
}

// splitOps cuts "main~2^2" into "main" and ["~2", "^2"].
func splitOps(expr string) (string, []string) {
	i := strings.IndexAny(expr, "~^")
	if i < 0 {
		return expr, nil
	}
	base, rest := expr[:i], expr[i:]
	var ops []string
	for rest != "" {
		end := 1
		if strings.HasPrefix(rest, "^{") {
			if c := strings.Index(rest, "}"); c > 0 {
				end = c + 1
			}
		} else {
			for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
				end++
			}
		}
		ops = append(ops, rest[:end])
		rest = rest[end:]
	}
	return base, ops
}

func resolveBase(r *repo.Repo, base, full string) (objects.ID, error) {
	if base == "@" {
		base = "HEAD"
	}
	if name, n, ok := strings.Cut(base, "@{"); ok {
		return resolveReflog(r, name, strings.TrimSuffix(n, "}"), full)
	}
	if base == "" {
		return objects.Zero, &NotFoundError{Expr: full}
	}
	if len(base) == 40 {
		if id, err := objects.ParseID(base); err == nil && r.ODB.Has(id) {
			return id, nil
		}
	}
	if id, ok, err := lookupRef(r, base); err != nil || ok {
		return id, err
	}
	id, err := r.ODB.Resolve(base)
	var amb *odb.AmbiguousError
	switch {
	case errors.As(err, &amb):
		return objects.Zero, err
	case err != nil:
		return objects.Zero, &NotFoundError{Expr: full}
	}
	return id, nil
}

// lookupRef tries the places git tries, in the same order.
func lookupRef(r *repo.Repo, name string) (objects.ID, bool, error) {
	if name == "HEAD" {
		h, err := r.Refs.Head()
		if err != nil {
			return objects.Zero, false, err
		}
		if h.Unborn() {
			return objects.Zero, false, fmt.Errorf("HEAD does not point at a commit yet: %q has no commits", h.Branch())
		}
		return h.ID, true, nil
	}
	for _, candidate := range []string{name, "refs/" + name, refs.TagPrefix + name, refs.BranchPrefix + name} {
		if refs.ValidName(candidate) != nil {
			continue
		}
		id, err := r.Refs.Read(candidate)
		if err == nil {
			return id, true, nil
		}
		if !errors.Is(err, refs.ErrNotFound) {
			return objects.Zero, false, err
		}
	}
	return objects.Zero, false, nil
}

func resolveReflog(r *repo.Repo, name, n, full string) (objects.ID, error) {
	idx, err := strconv.Atoi(n)
	if err != nil || idx < 0 {
		return objects.Zero, fmt.Errorf("bad reflog index in %q, only @{N} is supported", full)
	}
	logName := "HEAD"
	switch name {
	case "", "HEAD":
	case "stash":
		logName = "refs/stash"
	default:
		logName = refs.BranchPrefix + name
	}
	entries, err := r.Refs.ReadLog(logName)
	if err != nil {
		return objects.Zero, err
	}
	if idx >= len(entries) {
		return objects.Zero, fmt.Errorf("%s has only %d reflog entries, cannot use @{%d}", logName, len(entries), idx)
	}
	return entries[idx].New, nil
}

func apply(r *repo.Repo, id objects.ID, op, full string) (objects.ID, error) {
	if strings.HasPrefix(op, "^{") {
		return peelTo(r, id, strings.TrimSuffix(op[2:], "}"), full)
	}
	n := 1
	if len(op) > 1 {
		n, _ = strconv.Atoi(op[1:])
	}
	commitID, err := peel(r, id, objects.TypeCommit)
	if err != nil {
		return objects.Zero, err
	}
	if op[0] == '^' {
		if n == 0 {
			return commitID, nil
		}
		return parent(r, commitID, n, full)
	}
	for i := 0; i < n; i++ {
		if commitID, err = parent(r, commitID, 1, full); err != nil {
			return objects.Zero, err
		}
	}
	return commitID, nil
}

func parent(r *repo.Repo, id objects.ID, n int, full string) (objects.ID, error) {
	c, err := r.ODB.ReadCommit(id)
	if err != nil {
		return objects.Zero, err
	}
	if n > len(c.Parents) {
		return objects.Zero, fmt.Errorf("%q: commit %s has %d parent(s), no parent number %d",
			full, id.Short(), len(c.Parents), n)
	}
	return c.Parents[n-1], nil
}

func peelTo(r *repo.Repo, id objects.ID, kind, full string) (objects.ID, error) {
	switch kind {
	case "", "object":
		return id, nil
	case "commit":
		return peel(r, id, objects.TypeCommit)
	case "tree":
		commitID, err := peel(r, id, objects.TypeCommit)
		if err != nil {
			return peel(r, id, objects.TypeTree)
		}
		c, err := r.ODB.ReadCommit(commitID)
		if err != nil {
			return objects.Zero, err
		}
		return c.Tree, nil
	}
	return objects.Zero, fmt.Errorf("unsupported peel type in %q", full)
}

// peel follows annotated tags until it reaches an object of type want.
func peel(r *repo.Repo, id objects.ID, want objects.Type) (objects.ID, error) {
	for {
		o, err := r.ODB.Read(id)
		if err != nil {
			return objects.Zero, err
		}
		if o.Type() == want {
			return id, nil
		}
		tag, ok := o.(*objects.Tag)
		if !ok {
			return objects.Zero, fmt.Errorf("object %s is a %s, not a %s", id.Short(), o.Type(), want)
		}
		id = tag.Object
	}
}

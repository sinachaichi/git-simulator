package porcelain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// Errors returned by the tag functions.
var (
	ErrTagExists   = errors.New("tag already exists")
	ErrTagNotFound = errors.New("tag not found")
)

// TagInfo is one entry of the tag list.
type TagInfo struct {
	Name      string
	ID        objects.ID // what the ref points to: a commit or a tag object
	Annotated bool
	Subject   string // first line of the annotation, empty for lightweight tags
}

// CreateTag creates a tag at rev (HEAD when empty). A non-empty message
// makes it an annotated tag with its own object, otherwise the ref
// points straight at the commit.
func CreateTag(r *repo.Repo, name, rev, message string) (objects.ID, error) {
	ref := refs.TagPrefix + name
	if err := refs.ValidName(ref); err != nil {
		return objects.Zero, err
	}
	if r.Refs.Exists(ref) {
		return objects.Zero, fmt.Errorf("%w: %s", ErrTagExists, name)
	}
	if rev == "" {
		rev = "HEAD"
	}
	target, err := revparse.Resolve(r, rev)
	if err != nil {
		return objects.Zero, err
	}
	o, err := r.ODB.Read(target)
	if err != nil {
		return objects.Zero, err
	}
	id := target
	if msg := CleanMessage(message); msg != "" {
		tagger, err := r.Identity("committer")
		if err != nil {
			return objects.Zero, err
		}
		id, _, err = r.ODB.Write(&objects.Tag{Object: target, Kind: o.Type(), Name: name, Tagger: tagger, Message: msg})
		if err != nil {
			return objects.Zero, err
		}
	}
	return id, r.Refs.Update(ref, id)
}

// ListTags returns all tags sorted by name.
func ListTags(r *repo.Repo) ([]TagInfo, error) {
	list, err := r.Refs.List(refs.TagPrefix)
	if err != nil {
		return nil, err
	}
	out := make([]TagInfo, 0, len(list))
	for _, t := range list {
		info := TagInfo{Name: strings.TrimPrefix(t.Name, refs.TagPrefix), ID: t.ID}
		o, err := r.ODB.Read(t.ID)
		if err != nil {
			return nil, fmt.Errorf("tag %s: %w", info.Name, err)
		}
		if tag, ok := o.(*objects.Tag); ok {
			info.Annotated = true
			info.Subject = strings.SplitN(tag.Message, "\n", 2)[0]
		}
		out = append(out, info)
	}
	return out, nil
}

// DeleteTag removes a tag ref. The tag object, if any, stays in the
// object database until something else cleans it up, as in git.
func DeleteTag(r *repo.Repo, name string) (objects.ID, error) {
	ref := refs.TagPrefix + name
	id, err := r.Refs.Read(ref)
	if errors.Is(err, refs.ErrNotFound) {
		return objects.Zero, fmt.Errorf("%w: %s", ErrTagNotFound, name)
	}
	if err != nil {
		return objects.Zero, err
	}
	return id, r.Refs.Delete(ref)
}

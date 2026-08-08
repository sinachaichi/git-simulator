package merge

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/tree"
)

// ConflictKind says why a path could not be merged on its own.
type ConflictKind string

// The kinds of conflict.
const (
	ContentConflict ConflictKind = "content"       // both sides changed the same file
	AddAdd          ConflictKind = "add/add"       // both sides added the file with different content
	ModifyDelete    ConflictKind = "modify/delete" // one side edited what the other deleted
	ModeConflict    ConflictKind = "mode"          // both sides changed the file mode
	BinaryConflict  ConflictKind = "binary"        // both sides changed a binary file
)

// Conflict is a path that needs a decision from the user. Base, Ours and
// Theirs are the three versions, nil when a side does not have the file.
type Conflict struct {
	Path    string
	Kind    ConflictKind
	Base    *tree.File
	Ours    *tree.File
	Theirs  *tree.File
	Content []byte // what to write in the working tree, with markers for text
	Mode    uint32 // mode for the working tree file
}

// Result is the outcome of merging two trees.
type Result struct {
	Files     []tree.File // merged files without conflicts, sorted by path
	Conflicts []Conflict  // sorted by path
}

// Trees merges two sets of files that come from a common base. Clean
// text merges are written to the object store. Renames are not
// detected, so a renamed and edited file shows up as a delete plus an
// add.
func Trees(s *odb.Store, base, ours, theirs []tree.File, labels Labels) (Result, error) {
	b, o, t := byPath(base), byPath(ours), byPath(theirs)
	seen := map[string]bool{}
	var paths []string
	for _, m := range []map[string]tree.File{b, o, t} {
		for p := range m {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)

	var res Result
	for _, p := range paths {
		bf, inBase := b[p]
		of, inOurs := o[p]
		tf, inTheirs := t[p]
		switch {
		case inOurs == inTheirs && of == tf:
			if inOurs {
				res.Files = append(res.Files, of)
			}
		case inOurs == inBase && of == bf:
			if inTheirs {
				res.Files = append(res.Files, tf)
			}
		case inTheirs == inBase && tf == bf:
			if inOurs {
				res.Files = append(res.Files, of)
			}
		default:
			clean, conflict, err := mergePath(s, p, ptr(bf, inBase), ptr(of, inOurs), ptr(tf, inTheirs), labels)
			if err != nil {
				return Result{}, err
			}
			if conflict != nil {
				res.Conflicts = append(res.Conflicts, *conflict)
			} else {
				res.Files = append(res.Files, *clean)
			}
		}
	}
	if err := checkDirectoryFileClash(res); err != nil {
		return Result{}, err
	}
	return res, nil
}

func byPath(files []tree.File) map[string]tree.File {
	m := make(map[string]tree.File, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func ptr(f tree.File, ok bool) *tree.File {
	if !ok {
		return nil
	}
	return &f
}

// mergePath handles a path that both sides changed in different ways.
func mergePath(s *odb.Store, path string, base, ours, theirs *tree.File, labels Labels) (*tree.File, *Conflict, error) {
	conflict := func(kind ConflictKind, content []byte, mode uint32) (*tree.File, *Conflict, error) {
		return nil, &Conflict{Path: path, Kind: kind, Base: base, Ours: ours, Theirs: theirs, Content: content, Mode: mode}, nil
	}
	switch {
	case ours == nil: // deleted by us, changed by them
		data, err := blobData(s, theirs)
		if err != nil {
			return nil, nil, err
		}
		return conflict(ModifyDelete, data, theirs.Mode)
	case theirs == nil: // changed by us, deleted by them
		data, err := blobData(s, ours)
		if err != nil {
			return nil, nil, err
		}
		return conflict(ModifyDelete, data, ours.Mode)
	}

	mode, modeOK := mergeMode(base, ours, theirs)
	var baseData []byte
	if base != nil {
		var err error
		if baseData, err = blobData(s, base); err != nil {
			return nil, nil, err
		}
	}
	oursData, err := blobData(s, ours)
	if err != nil {
		return nil, nil, err
	}
	theirsData, err := blobData(s, theirs)
	if err != nil {
		return nil, nil, err
	}

	if ours.ID == theirs.ID { // same content, only the mode is in question
		if !modeOK {
			return conflict(ModeConflict, oursData, ours.Mode)
		}
		return &tree.File{Path: path, Mode: mode, ID: ours.ID}, nil, nil
	}
	if diff.IsBinary(baseData) || diff.IsBinary(oursData) || diff.IsBinary(theirsData) {
		return conflict(BinaryConflict, oursData, ours.Mode)
	}
	text, n := Text(string(baseData), string(oursData), string(theirsData), labels)
	if n > 0 {
		kind := ContentConflict
		if base == nil {
			kind = AddAdd
		}
		return conflict(kind, []byte(text), ours.Mode)
	}
	if !modeOK {
		return conflict(ModeConflict, []byte(text), ours.Mode)
	}
	id, _, err := s.Write(&objects.Blob{Data: []byte(text)})
	if err != nil {
		return nil, nil, err
	}
	return &tree.File{Path: path, Mode: mode, ID: id}, nil, nil
}

// mergeMode picks the mode when only one side changed it. ok is false
// when both changed it to different values.
func mergeMode(base, ours, theirs *tree.File) (uint32, bool) {
	switch {
	case ours.Mode == theirs.Mode:
		return ours.Mode, true
	case base != nil && ours.Mode == base.Mode:
		return theirs.Mode, true
	case base != nil && theirs.Mode == base.Mode:
		return ours.Mode, true
	}
	return ours.Mode, false
}

func blobData(s *odb.Store, f *tree.File) ([]byte, error) {
	b, err := s.ReadBlob(f.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Path, err)
	}
	return b.Data, nil
}

// checkDirectoryFileClash rejects results where one path is a file and
// also a directory. Git can resolve that case, we do not.
func checkDirectoryFileClash(res Result) error {
	var all []string
	for _, f := range res.Files {
		all = append(all, f.Path)
	}
	for _, c := range res.Conflicts {
		all = append(all, c.Path)
	}
	sort.Strings(all)
	for i := 1; i < len(all); i++ {
		if strings.HasPrefix(all[i], all[i-1]+"/") {
			return fmt.Errorf("%s is a file on one side and a directory on the other, which is not supported", all[i-1])
		}
	}
	return nil
}

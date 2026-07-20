package diff

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

// File is the difference of one path between two states. A zero mode
// means the file does not exist on that side.
type File struct {
	Path    string
	OldMode uint32
	NewMode uint32
	OldID   objects.ID
	NewID   objects.ID
	Old     []byte
	New     []byte
}

// IsBinary uses git's rule: a NUL byte in the first 8000 bytes.
func IsBinary(data []byte) bool {
	if len(data) > 8000 {
		data = data[:8000]
	}
	return bytes.IndexByte(data, 0) >= 0
}

// Render returns the patch for one file, in the layout of `git diff`.
// abbrev shortens object ids for the "index" line.
func (f File) Render(abbrev func(objects.ID) string, context int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", f.Path, f.Path)
	added, deleted := f.OldMode == 0, f.NewMode == 0
	switch {
	case added:
		fmt.Fprintf(&b, "new file mode %06o\n", f.NewMode)
	case deleted:
		fmt.Fprintf(&b, "deleted file mode %06o\n", f.OldMode)
	case f.OldMode != f.NewMode:
		fmt.Fprintf(&b, "old mode %06o\nnew mode %06o\n", f.OldMode, f.NewMode)
	}
	if f.OldID == f.NewID {
		return b.String() // only the mode changed
	}
	fmt.Fprintf(&b, "index %s..%s", abbrevOrZero(abbrev, f.OldID), abbrevOrZero(abbrev, f.NewID))
	if !added && !deleted && f.OldMode == f.NewMode {
		fmt.Fprintf(&b, " %06o", f.NewMode)
	}
	b.WriteByte('\n')

	oldName, newName := "a/"+f.Path, "b/"+f.Path
	if added {
		oldName = "/dev/null"
	}
	if deleted {
		newName = "/dev/null"
	}
	if IsBinary(f.Old) || IsBinary(f.New) {
		fmt.Fprintf(&b, "Binary files %s and %s differ\n", oldName, newName)
		return b.String()
	}
	hunks := Hunks(Lines(SplitLines(string(f.Old)), SplitLines(string(f.New))), context)
	if len(hunks) == 0 {
		return b.String() // an empty file was added or removed
	}
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", oldName, newName)
	for _, h := range hunks {
		b.WriteString(h.String())
	}
	return b.String()
}

func abbrevOrZero(abbrev func(objects.ID) string, id objects.ID) string {
	if id.IsZero() {
		return strings.Repeat("0", len(abbrev(id)))
	}
	return abbrev(id)
}

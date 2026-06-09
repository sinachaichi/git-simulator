package objects

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// File modes used in tree entries.
const (
	ModeFile       uint32 = 0o100644
	ModeExecutable uint32 = 0o100755
	ModeDir        uint32 = 0o040000
)

// TreeEntry is one name in a directory listing.
type TreeEntry struct {
	Mode uint32
	Name string
	ID   ID
}

// IsDir reports whether the entry points to another tree.
func (e TreeEntry) IsDir() bool { return e.Mode == ModeDir }

// Tree is a directory snapshot.
type Tree struct {
	Entries []TreeEntry
}

// Type implements Object.
func (t *Tree) Type() Type { return TypeTree }

// sortKey mimics git: directories sort as if their name ended with a slash.
func (e TreeEntry) sortKey() string {
	if e.IsDir() {
		return e.Name + "/"
	}
	return e.Name
}

// Sort puts the entries in the order git requires.
func (t *Tree) Sort() {
	sort.Slice(t.Entries, func(i, j int) bool {
		return t.Entries[i].sortKey() < t.Entries[j].sortKey()
	})
}

// Body implements Object. Entries are written in the order they have, so
// call Sort first when building a tree by hand.
func (t *Tree) Body() []byte {
	var buf bytes.Buffer
	for _, e := range t.Entries {
		// Git writes the mode in octal with no leading zero, so a
		// directory is "40000".
		buf.WriteString(strconv.FormatUint(uint64(e.Mode), 8))
		buf.WriteByte(' ')
		buf.WriteString(e.Name)
		buf.WriteByte(0)
		buf.Write(e.ID[:])
	}
	return buf.Bytes()
}

// ParseTree decodes the body of a tree object.
func ParseTree(body []byte) (*Tree, error) {
	t := &Tree{}
	for len(body) > 0 {
		sp := bytes.IndexByte(body, ' ')
		if sp < 0 {
			return nil, fmt.Errorf("corrupt tree: missing mode separator")
		}
		mode, err := strconv.ParseUint(string(body[:sp]), 8, 32)
		if err != nil {
			return nil, fmt.Errorf("corrupt tree: bad mode %q", body[:sp])
		}
		body = body[sp+1:]
		nul := bytes.IndexByte(body, 0)
		if nul < 0 || len(body) < nul+1+20 {
			return nil, fmt.Errorf("corrupt tree: truncated entry")
		}
		name := string(body[:nul])
		if name == "" || strings.Contains(name, "/") {
			return nil, fmt.Errorf("corrupt tree: bad entry name %q", name)
		}
		var id ID
		copy(id[:], body[nul+1:nul+21])
		t.Entries = append(t.Entries, TreeEntry{Mode: uint32(mode), Name: name, ID: id})
		body = body[nul+21:]
	}
	return t, nil
}

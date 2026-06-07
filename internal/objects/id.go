// Package objects implements the four git object types with the same byte
// layout and hashing as real git, so ids match for the same content.
package objects

import (
	"crypto/sha1" //nolint:gosec // git object ids are sha-1 by definition
	"encoding/hex"
	"fmt"
	"strconv"
)

// Type is the kind of a git object.
type Type string

// The four object types.
const (
	TypeBlob   Type = "blob"
	TypeTree   Type = "tree"
	TypeCommit Type = "commit"
	TypeTag    Type = "tag"
)

// ID is a sha-1 object id.
type ID [20]byte

// Zero is the all-zero id. Git uses it for "no object".
var Zero ID

// ParseID parses a full 40 character hex id.
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 40 {
		return id, fmt.Errorf("invalid object id %q: want 40 hex characters", s)
	}
	if _, err := hex.Decode(id[:], []byte(s)); err != nil {
		return id, fmt.Errorf("invalid object id %q: %w", s, err)
	}
	return id, nil
}

// String returns the 40 character hex form.
func (id ID) String() string { return hex.EncodeToString(id[:]) }

// Short returns the first 7 characters, like git's default abbreviation.
func (id ID) Short() string { return id.String()[:7] }

// IsZero reports whether id is the all-zero id.
func (id ID) IsZero() bool { return id == Zero }

// Object is anything that can be stored in the object database.
type Object interface {
	Type() Type
	// Body returns the content without the "<type> <size>\0" header.
	Body() []byte
}

// Serialize returns the object with its header, which is what gets hashed
// and, after zlib compression, written to disk.
func Serialize(o Object) []byte {
	body := o.Body()
	header := string(o.Type()) + " " + strconv.Itoa(len(body)) + "\x00"
	return append([]byte(header), body...)
}

// HashOf returns the id of an object without storing it.
func HashOf(o Object) ID {
	return sha1.Sum(Serialize(o)) //nolint:gosec // see above
}

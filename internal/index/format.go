// Package index reads and writes the staging area. It uses git's binary
// index format, version 2, so real git can read what we write.
package index

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // the index checksum is sha-1 by definition
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

// Entry is one staged file. Stage is 0 for a normal entry and 1, 2 or 3
// for the base, ours and theirs side of a merge conflict.
type Entry struct {
	Path  string
	Mode  uint32
	ID    objects.ID
	Stage uint8
	Size  uint32
	MTime time.Time
}

const (
	signature   = "DIRC"
	version     = 2
	fixedSize   = 62 // bytes before the path in one entry
	maxNameSize = 0xFFF
)

// Marshal encodes entries, which must already be sorted by path and stage.
func Marshal(entries []Entry) []byte {
	var buf bytes.Buffer
	buf.WriteString(signature)
	put32(&buf, version)
	put32(&buf, uint32(len(entries)))
	for _, e := range entries {
		start := buf.Len()
		sec, nsec := uint32(e.MTime.Unix()), uint32(e.MTime.Nanosecond())
		put32(&buf, sec) // ctime, we do not track it separately
		put32(&buf, nsec)
		put32(&buf, sec)
		put32(&buf, nsec)
		put32(&buf, 0) // dev
		put32(&buf, 0) // ino
		put32(&buf, e.Mode)
		put32(&buf, 0) // uid
		put32(&buf, 0) // gid
		put32(&buf, e.Size)
		buf.Write(e.ID[:])
		nameLen := len(e.Path)
		if nameLen > maxNameSize {
			nameLen = maxNameSize
		}
		flags := uint16(e.Stage)<<12 | uint16(nameLen)
		buf.Write([]byte{byte(flags >> 8), byte(flags)})
		buf.WriteString(e.Path)
		// Pad with 1 to 8 NULs so the entry length is a multiple of 8.
		pad := 8 - (buf.Len()-start)%8
		buf.Write(make([]byte, pad))
	}
	sum := sha1.Sum(buf.Bytes()) //nolint:gosec // see above
	buf.Write(sum[:])
	return buf.Bytes()
}

func put32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}

// Unmarshal decodes an index file and verifies its checksum.
func Unmarshal(data []byte) ([]Entry, error) {
	if len(data) < 12+20 {
		return nil, errors.New("index file is too short")
	}
	body, trailer := data[:len(data)-20], data[len(data)-20:]
	if sum := sha1.Sum(body); !bytes.Equal(sum[:], trailer) { //nolint:gosec // see above
		return nil, errors.New("index checksum does not match, the file is corrupt")
	}
	if string(body[:4]) != signature {
		return nil, errors.New("not an index file (bad signature)")
	}
	if v := binary.BigEndian.Uint32(body[4:8]); v != version {
		return nil, fmt.Errorf("unsupported index version %d", v)
	}
	count := int(binary.BigEndian.Uint32(body[8:12]))
	rest := body[12:]
	entries := make([]Entry, 0, count)
	for i := 0; i < count; i++ {
		if len(rest) < fixedSize {
			return nil, errors.New("index is truncated")
		}
		e := Entry{
			MTime: time.Unix(int64(binary.BigEndian.Uint32(rest[8:12])), int64(binary.BigEndian.Uint32(rest[12:16]))),
			Mode:  binary.BigEndian.Uint32(rest[24:28]),
			Size:  binary.BigEndian.Uint32(rest[36:40]),
		}
		copy(e.ID[:], rest[40:60])
		flags := binary.BigEndian.Uint16(rest[60:62])
		e.Stage = uint8(flags >> 12 & 3)
		nameLen := int(flags & maxNameSize)
		end := bytes.IndexByte(rest[fixedSize:], 0)
		if end < 0 || (nameLen < maxNameSize && end != nameLen) {
			return nil, errors.New("index entry has a bad path")
		}
		e.Path = string(rest[fixedSize : fixedSize+end])
		size := (fixedSize + end + 8) &^ 7
		if len(rest) < size {
			return nil, errors.New("index is truncated")
		}
		rest = rest[size:]
		entries = append(entries, e)
	}
	return entries, nil
}

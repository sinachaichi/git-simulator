package objects

import (
	"bytes"
	"fmt"
	"strconv"
)

// Parse decodes a serialized object, header included.
func Parse(data []byte) (Object, error) {
	nul := bytes.IndexByte(data, 0)
	if nul < 0 {
		return nil, fmt.Errorf("corrupt object: no header")
	}
	kind, sizeText, ok := bytes.Cut(data[:nul], []byte(" "))
	if !ok {
		return nil, fmt.Errorf("corrupt object: bad header %q", data[:nul])
	}
	size, err := strconv.Atoi(string(sizeText))
	if err != nil || size < 0 {
		return nil, fmt.Errorf("corrupt object: bad size %q", sizeText)
	}
	body := data[nul+1:]
	if len(body) != size {
		return nil, fmt.Errorf("corrupt object: header says %d bytes, found %d", size, len(body))
	}
	switch Type(kind) {
	case TypeBlob:
		return &Blob{Data: body}, nil
	case TypeTree:
		return ParseTree(body)
	case TypeCommit:
		return ParseCommit(body)
	case TypeTag:
		return ParseTag(body)
	}
	return nil, fmt.Errorf("unknown object type %q", kind)
}

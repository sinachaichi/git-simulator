package objects

import (
	"fmt"
	"strings"
)

// Tag is an annotated tag object. Lightweight tags are plain refs and do
// not use this type.
type Tag struct {
	Object  ID
	Kind    Type
	Name    string
	Tagger  Signature
	Message string
}

// Type implements Object.
func (t *Tag) Type() Type { return TypeTag }

// Body implements Object.
func (t *Tag) Body() []byte {
	return []byte(fmt.Sprintf("object %s\ntype %s\ntag %s\ntagger %s\n\n%s",
		t.Object, t.Kind, t.Name, t.Tagger, t.Message))
}

// ParseTag decodes the body of a tag object.
func ParseTag(body []byte) (*Tag, error) {
	head, msg, _ := strings.Cut(string(body), "\n\n")
	t := &Tag{Message: msg}
	var haveObject, haveName bool
	for _, line := range strings.Split(head, "\n") {
		key, val, _ := strings.Cut(line, " ")
		var err error
		switch key {
		case "object":
			t.Object, err = ParseID(val)
			haveObject = true
		case "type":
			t.Kind = Type(val)
		case "tag":
			t.Name = val
			haveName = true
		case "tagger":
			t.Tagger, err = ParseSignature(val)
		}
		if err != nil {
			return nil, fmt.Errorf("corrupt tag: %w", err)
		}
	}
	if !haveObject || !haveName || t.Kind == "" {
		return nil, fmt.Errorf("corrupt tag: missing object, type or name")
	}
	return t, nil
}

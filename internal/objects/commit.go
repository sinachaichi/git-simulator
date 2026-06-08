package objects

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signature is an author, committer or tagger line.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// String formats the signature as git does: "Name <email> 1700000000 +0200".
func (s Signature) String() string {
	_, offset := s.When.Zone()
	sign := '+'
	if offset < 0 {
		sign = '-'
		offset = -offset
	}
	return fmt.Sprintf("%s <%s> %d %c%02d%02d", s.Name, s.Email, s.When.Unix(),
		sign, offset/3600, offset%3600/60)
}

// ParseSignature reads the text after "author ", "committer " or "tagger ".
func ParseSignature(line string) (Signature, error) {
	lt := strings.LastIndex(line, " <")
	gt := strings.LastIndex(line, "> ")
	if lt < 0 || gt < lt {
		return Signature{}, fmt.Errorf("bad signature %q", line)
	}
	fields := strings.Fields(line[gt+2:])
	if len(fields) != 2 || len(fields[1]) != 5 {
		return Signature{}, fmt.Errorf("bad signature date in %q", line)
	}
	unix, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return Signature{}, fmt.Errorf("bad signature time in %q", line)
	}
	hours, err1 := strconv.Atoi(fields[1][1:3])
	mins, err2 := strconv.Atoi(fields[1][3:])
	if err1 != nil || err2 != nil || (fields[1][0] != '+' && fields[1][0] != '-') {
		return Signature{}, fmt.Errorf("bad signature zone in %q", line)
	}
	offset := hours*3600 + mins*60
	if fields[1][0] == '-' {
		offset = -offset
	}
	return Signature{
		Name:  line[:lt],
		Email: line[lt+2 : gt],
		When:  time.Unix(unix, 0).In(time.FixedZone("", offset)),
	}, nil
}

// Commit is a snapshot (a tree) plus history (parents) and metadata.
type Commit struct {
	Tree      ID
	Parents   []ID
	Author    Signature
	Committer Signature
	Message   string
}

// Type implements Object.
func (c *Commit) Type() Type { return TypeCommit }

// Body implements Object.
func (c *Commit) Body() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "tree %s\n", c.Tree)
	for _, p := range c.Parents {
		fmt.Fprintf(&b, "parent %s\n", p)
	}
	fmt.Fprintf(&b, "author %s\n", c.Author)
	fmt.Fprintf(&b, "committer %s\n\n", c.Committer)
	b.WriteString(c.Message)
	return b.Bytes()
}

// Subject returns the first line of the message.
func (c *Commit) Subject() string {
	return strings.SplitN(c.Message, "\n", 2)[0]
}

// ParseCommit decodes the body of a commit object.
func ParseCommit(body []byte) (*Commit, error) {
	head, msg, found := strings.Cut(string(body), "\n\n")
	if !found {
		// A commit with no message ends right after the committer line.
		head, msg = strings.TrimSuffix(string(body), "\n"), ""
	}
	c := &Commit{Message: msg}
	var haveTree, haveAuthor, haveCommitter bool
	for _, line := range strings.Split(head, "\n") {
		key, val, _ := strings.Cut(line, " ")
		var err error
		switch key {
		case "tree":
			c.Tree, err = ParseID(val)
			haveTree = true
		case "parent":
			var p ID
			p, err = ParseID(val)
			c.Parents = append(c.Parents, p)
		case "author":
			c.Author, err = ParseSignature(val)
			haveAuthor = true
		case "committer":
			c.Committer, err = ParseSignature(val)
			haveCommitter = true
		}
		if err != nil {
			return nil, fmt.Errorf("corrupt commit: %w", err)
		}
	}
	if !haveTree || !haveAuthor || !haveCommitter {
		return nil, fmt.Errorf("corrupt commit: missing tree, author or committer")
	}
	return c, nil
}

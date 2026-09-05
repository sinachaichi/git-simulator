// Package repl is the interactive shell: a prompt that runs hood
// commands, with history, completion and a few helpers for editing files.
package repl

import (
	"errors"
	"strings"
)

// SplitArgs splits a line into words. It understands single quotes,
// double quotes and backslash escapes, like a basic shell.
func SplitArgs(line string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inWord := false
	var quote rune
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				args = append(args, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote")
	}
	if escaped {
		return nil, errors.New("line ends with a backslash")
	}
	if inWord {
		args = append(args, cur.String())
	}
	return args, nil
}

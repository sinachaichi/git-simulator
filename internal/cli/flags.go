package cli

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// usageError marks a mistake in how a command was called.
type usageError struct {
	msg string
}

func (e *usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// stringList is a flag that can be given several times, like commit -m.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

var shortGroup = regexp.MustCompile(`^-[a-zA-Z]{2,}$`)

// parseFlags is flag.Parse with two git habits added: flags may come
// after positional arguments, and short flags can be grouped (-am).
// It returns the positional arguments.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case shortGroup.MatchString(a) && fs.Lookup(a[1:]) == nil:
			for _, letter := range a[1:] {
				flags = append(flags, "-"+string(letter))
			}
			// The last letter may take a value, as in -am "message".
			last := a[len(a)-1:]
			if f := fs.Lookup(last); f != nil && !isBool(f) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(a, "-") && a != "-":
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") {
				continue
			}
			if f := fs.Lookup(name); f != nil && !isBool(f) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			rest = append(rest, a)
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, usagef("%s", strings.TrimPrefix(err.Error(), "flag provided but not defined: "))
	}
	return rest, nil
}

func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

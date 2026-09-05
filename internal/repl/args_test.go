package repl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/repl"
)

func TestSplitArgs(t *testing.T) {
	tests := map[string][]string{
		``:                         nil,
		`   `:                      nil,
		`status`:                   {"status"},
		`commit -m "first commit"`: {"commit", "-m", "first commit"},
		`commit -m 'it''s'`:        {"commit", "-m", "its"},
		`commit -m "say \"hi\""`:   {"commit", "-m", `say "hi"`},
		`add  a.txt   b.txt`:       {"add", "a.txt", "b.txt"},
		`write f ""`:               {"write", "f", ""},
		`add my\ file`:             {"add", "my file"},
		`echo 'a "b" c'`:           {"echo", `a "b" c`},
		`tab	separated`:            {"tab", "separated"},
	}
	for line, want := range tests {
		got, err := repl.SplitArgs(line)
		assert.NoError(t, err, line)
		assert.Equal(t, want, got, line)
	}
}

func TestSplitArgsErrors(t *testing.T) {
	_, err := repl.SplitArgs(`commit -m "open`)
	assert.ErrorContains(t, err, "unclosed quote")
	_, err = repl.SplitArgs(`add a\`)
	assert.ErrorContains(t, err, "backslash")
}

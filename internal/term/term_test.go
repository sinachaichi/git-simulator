package term_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/term"
)

func TestStyleWrapsOnlyWhenEnabled(t *testing.T) {
	on, off := term.Style{Enabled: true}, term.Style{}
	assert.Equal(t, "\x1b[31mboom\x1b[0m", on.Red("boom"))
	assert.Equal(t, "boom", off.Red("boom"))
	assert.Equal(t, "", on.Green(""), "empty text stays empty")
}

func TestDetectIsOffForBuffersAndNoColor(t *testing.T) {
	assert.False(t, term.Detect(&bytes.Buffer{}, false).Enabled)
	t.Setenv("NO_COLOR", "1")
	assert.False(t, term.Detect(&bytes.Buffer{}, false).Enabled)
	assert.False(t, term.Detect(&bytes.Buffer{}, true).Enabled)
}

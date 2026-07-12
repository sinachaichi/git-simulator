// Package term handles colored output. Colors are off when the output is
// not a terminal, when NO_COLOR is set, or when the user asks for none.
package term

import (
	"io"
	"os"
)

// Style wraps text in ANSI escape codes when Enabled is true.
type Style struct {
	Enabled bool
}

// Codes for the styles we use.
const (
	reset  = "\x1b[0m"
	bold   = "1"
	dim    = "2"
	red    = "31"
	green  = "32"
	yellow = "33"
	blue   = "34"
	purple = "35"
	cyan   = "36"
)

func (s Style) wrap(code, text string) string {
	if !s.Enabled || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + reset
}

// Bold makes text bold.
func (s Style) Bold(t string) string { return s.wrap(bold, t) }

// Dim makes text faint.
func (s Style) Dim(t string) string { return s.wrap(dim, t) }

// Red colors text red.
func (s Style) Red(t string) string { return s.wrap(red, t) }

// Green colors text green.
func (s Style) Green(t string) string { return s.wrap(green, t) }

// Yellow colors text yellow.
func (s Style) Yellow(t string) string { return s.wrap(yellow, t) }

// Blue colors text blue.
func (s Style) Blue(t string) string { return s.wrap(blue, t) }

// Purple colors text purple.
func (s Style) Purple(t string) string { return s.wrap(purple, t) }

// Cyan colors text cyan.
func (s Style) Cyan(t string) string { return s.wrap(cyan, t) }

// Detect decides whether w should get colors. noColor is the value of a
// --no-color flag.
func Detect(w io.Writer, noColor bool) Style {
	if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return Style{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return Style{}
	}
	info, err := f.Stat()
	if err != nil {
		return Style{}
	}
	return Style{Enabled: info.Mode()&os.ModeCharDevice != 0}
}

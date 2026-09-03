package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterh/liner"
)

// lineReader hides the difference between a terminal and a pipe.
type lineReader interface {
	ReadLine(prompt string) (string, error)
	AddHistory(line string)
	Close()
}

func isTerminal(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0 && liner.TerminalSupported()
}

// plainReader reads lines from a pipe or a test buffer.
type plainReader struct {
	r   *bufio.Reader
	out io.Writer
}

func (p *plainReader) ReadLine(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	line, err := p.r.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", err
	}
	fmt.Fprintln(p.out, strings.TrimRight(line, "\r\n"))
	return strings.TrimRight(line, "\r\n"), nil
}

func (p *plainReader) AddHistory(string) {}
func (p *plainReader) Close()            {}

// terminalReader is liner with history saved on disk and completion.
type terminalReader struct {
	l    *liner.State
	file string
}

func newTerminalReader(s *Shell) *terminalReader {
	l := liner.NewLiner()
	l.SetCtrlCAborts(true)
	l.SetTabCompletionStyle(liner.TabPrints)
	l.SetCompleter(func(line string) []string { return s.complete(line) })
	t := &terminalReader{l: l, file: historyFile()}
	if t.file != "" {
		if f, err := os.Open(t.file); err == nil {
			_, _ = l.ReadHistory(f)
			_ = f.Close()
		}
	}
	return t
}

func (t *terminalReader) ReadLine(prompt string) (string, error) { return t.l.Prompt(prompt) }
func (t *terminalReader) AddHistory(line string)                 { t.l.AppendHistory(line) }

func (t *terminalReader) Close() {
	if t.file != "" {
		if err := os.MkdirAll(dirOf(t.file), 0o755); err == nil {
			if f, err := os.Create(t.file); err == nil {
				_, _ = t.l.WriteHistory(f)
				_ = f.Close()
			}
		}
	}
	_ = t.l.Close()
}

func dirOf(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[:i]
	}
	return "."
}

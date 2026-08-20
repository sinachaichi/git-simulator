package refs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/events"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

// LogEntry is one line of a reflog.
type LogEntry struct {
	Old     objects.ID
	New     objects.ID
	Who     objects.Signature
	Message string
}

func (s *Store) logPath(name string) string {
	return filepath.Join(s.Dir, "logs", filepath.FromSlash(name))
}

// AppendLog adds an entry to the reflog of a ref (or of "HEAD"). The line
// format is git's: "<old> <new> <who> <time> <zone>\t<message>".
func (s *Store) AppendLog(name string, e LogEntry) error {
	p := s.logPath(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	msg := strings.ReplaceAll(e.Message, "\n", " ")
	_, err = fmt.Fprintf(f, "%s %s %s\t%s\n", e.Old, e.New, e.Who, msg)
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.ReflogAppended, Subject: name, Old: e.Old.String(), New: e.New.String(), Detail: msg})
	return nil
}

// ReadLog returns the reflog of a ref, newest entry first. A ref without
// a log gives an empty list.
func (s *Store) ReadLog(name string) ([]LogEntry, error) {
	data, err := os.ReadFile(s.logPath(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		e, err := parseLogLine(line)
		if err != nil {
			return nil, fmt.Errorf("reflog of %s: %w", name, err)
		}
		entries = append(entries, e)
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

func parseLogLine(line string) (LogEntry, error) {
	head, msg, _ := strings.Cut(line, "\t")
	if len(head) < 83 {
		return LogEntry{}, fmt.Errorf("corrupt line %q", line)
	}
	oldID, err := objects.ParseID(head[:40])
	if err != nil {
		return LogEntry{}, err
	}
	newID, err := objects.ParseID(head[41:81])
	if err != nil {
		return LogEntry{}, err
	}
	who, err := objects.ParseSignature(head[82:])
	if err != nil {
		return LogEntry{}, err
	}
	return LogEntry{Old: oldID, New: newID, Who: who, Message: msg}, nil
}

// MoveLog renames the reflog of a ref. A missing log is not an error.
func (s *Store) MoveLog(from, to string) error {
	src, dst := s.logPath(from), s.logPath(to)
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// DeleteLog removes the reflog of a ref and empty parent directories.
func (s *Store) DeleteLog(name string) error {
	p := s.logPath(name)
	if err := os.Remove(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	stop := filepath.Join(s.Dir, "logs", "refs", "heads")
	for dir := filepath.Dir(p); dir != stop && strings.HasPrefix(dir, stop); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// ReplaceLog overwrites the reflog of a ref with entries, newest first.
// An empty list removes the log. Stash uses it to drop one entry.
func (s *Store) ReplaceLog(name string, entries []LogEntry) error {
	if len(entries) == 0 {
		return s.DeleteLog(name)
	}
	p := s.logPath(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		fmt.Fprintf(&b, "%s %s %s\t%s\n", e.Old, e.New, e.Who, strings.ReplaceAll(e.Message, "\n", " "))
	}
	return os.WriteFile(p, []byte(b.String()), 0o644)
}

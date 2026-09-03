package lessons

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Progress is what is saved between sessions.
type Progress struct {
	Active  string                  `json:"active"`
	Lessons map[string]LessonRecord `json:"lessons"`
}

// LessonRecord is the saved state of one lesson.
type LessonRecord struct {
	Step int  `json:"step"` // number of finished steps
	Done bool `json:"done"`
}

// Store keeps the progress file and the lesson workspaces below Home.
type Store struct {
	Home string
}

// DefaultHome is $HOOD_HOME, or a folder in the user's data directory.
func DefaultHome() (string, error) {
	if h := os.Getenv("HOOD_HOME"); h != "" {
		return h, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hood"), nil
}

func (s Store) file() string { return filepath.Join(s.Home, "progress.json") }

// Workspace is the folder of the repo for a lesson.
func (s Store) Workspace(name string) string {
	return filepath.Join(s.Home, "lessons", name)
}

// Load reads the progress. A missing file is an empty progress.
func (s Store) Load() (Progress, error) {
	p := Progress{Lessons: map[string]LessonRecord{}}
	data, err := os.ReadFile(s.file())
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return Progress{Lessons: map[string]LessonRecord{}}, err
	}
	if p.Lessons == nil {
		p.Lessons = map[string]LessonRecord{}
	}
	return p, nil
}

// Save writes the progress through a temp file, so a crash cannot leave
// half a file behind.
func (s Store) Save(p Progress) error {
	if err := os.MkdirAll(s.Home, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.file() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.file())
}

package lessons_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/lessons"
)

func TestProgressRoundTrip(t *testing.T) {
	s := lessons.Store{Home: filepath.Join(t.TempDir(), "home")}
	p, err := s.Load()
	require.NoError(t, err)
	assert.Empty(t, p.Active)
	assert.NotNil(t, p.Lessons)

	p.Active = "branching"
	p.Lessons["branching"] = lessons.LessonRecord{Step: 2}
	p.Lessons["first-commit"] = lessons.LessonRecord{Step: 3, Done: true}
	require.NoError(t, s.Save(p))

	got, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, p, got)
	assert.NoFileExists(t, filepath.Join(s.Home, "progress.json.tmp"))
}

func TestCorruptProgressIsAnError(t *testing.T) {
	s := lessons.Store{Home: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(s.Home, "progress.json"), []byte("{not json"), 0o644))
	_, err := s.Load()
	assert.Error(t, err)
}

func TestWorkspaceIsInsideHome(t *testing.T) {
	s := lessons.Store{Home: "/h"}
	assert.Equal(t, filepath.Join("/h", "lessons", "branching"), s.Workspace("branching"))
}

func TestDefaultHomeUsesEnvironment(t *testing.T) {
	t.Setenv("HOOD_HOME", "/custom/place")
	h, err := lessons.DefaultHome()
	require.NoError(t, err)
	assert.Equal(t, "/custom/place", h)
}

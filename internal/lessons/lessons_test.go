package lessons_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/lessons"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// start prepares a lesson in a temp home and returns its repo.
func start(t *testing.T, name string) (lessons.Lesson, *repo.Repo) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	l, err := lessons.Find(name)
	require.NoError(t, err)
	r, err := lessons.Prepare(lessons.Store{Home: t.TempDir()}, l)
	require.NoError(t, err)
	return l, r
}

func do(t *testing.T, err error) {
	t.Helper()
	require.NoError(t, err)
}

func write(t *testing.T, r *repo.Repo, path, content string) {
	t.Helper()
	require.NoError(t, r.Work.WriteFile(path, []byte(content), 0o644))
}

func add(t *testing.T, r *repo.Repo, paths ...string) {
	t.Helper()
	_, err := porcelain.Add(r, paths)
	require.NoError(t, err)
}

func commit(t *testing.T, r *repo.Repo, msg string) {
	t.Helper()
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: msg})
	require.NoError(t, err)
}

func sw(t *testing.T, r *repo.Repo, name string, create bool) {
	t.Helper()
	_, err := porcelain.Switch(r, name, porcelain.SwitchOptions{Create: create})
	require.NoError(t, err)
}

// progress remembers how many steps were finished, like the saved file
// does. Later steps may undo what earlier ones needed, so Check always
// continues from here and never starts at zero again.
type progress struct {
	t    *testing.T
	l    lessons.Lesson
	r    *repo.Repo
	done int
}

func track(t *testing.T, l lessons.Lesson, r *repo.Repo) *progress {
	return &progress{t: t, l: l, r: r}
}

// at checks that the lesson is waiting for the given 1-based step, and
// 0 means finished.
func (p *progress) at(next int) lessons.Outcome {
	p.t.Helper()
	out := lessons.Check(p.l, p.r, p.done)
	p.done += len(out.Completed)
	require.Equal(p.t, next, out.Next, "missing: %s", out.Missing)
	assert.Equal(p.t, next == 0, out.Done)
	if next > 0 {
		assert.NotEmpty(p.t, out.Missing)
	}
	return out
}

// expect checks that the lesson is at the given open step.
func expect(t *testing.T, l lessons.Lesson, r *repo.Repo, from, next int) lessons.Outcome {
	t.Helper()
	out := lessons.Check(l, r, from)
	require.Equal(t, next, out.Next, "missing: %s", out.Missing)
	if next > 0 {
		assert.NotEmpty(t, out.Missing)
		assert.False(t, out.Done)
	}
	return out
}

func TestCatalogIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range lessons.All() {
		assert.False(t, seen[l.Name], "duplicate lesson %s", l.Name)
		seen[l.Name] = true
		assert.NotEmpty(t, l.Title)
		assert.NotEmpty(t, l.Summary)
		assert.NotNil(t, l.Setup)
		require.NotEmpty(t, l.Steps, l.Name)
		for i, s := range l.Steps {
			assert.NotEmpty(t, s.Task, "%s step %d", l.Name, i+1)
			assert.NotEmpty(t, s.Hint, "%s step %d", l.Name, i+1)
			assert.NotNil(t, s.Check, "%s step %d", l.Name, i+1)
		}
	}
	assert.GreaterOrEqual(t, len(seen), 6)
}

func TestFindUnknownLessonListsTheKnownOnes(t *testing.T) {
	_, err := lessons.Find("nope")
	assert.ErrorContains(t, err, "first-commit")
}

func TestEveryLessonStartsUnsolved(t *testing.T) {
	for _, l := range lessons.All() {
		_, r := start(t, l.Name)
		out := lessons.Check(l, r, 0)
		assert.False(t, out.Done, l.Name)
		assert.Equal(t, 1, out.Next, l.Name)
	}
}

func TestFirstCommitLesson(t *testing.T) {
	l, r := start(t, "first-commit")
	expect(t, l, r, 0, 1)
	add(t, r, "README.md")
	expect(t, l, r, 0, 2)
	commit(t, r, "first commit")
	expect(t, l, r, 0, 3)
	write(t, r, "README.md", "# My project\nmore\n")
	add(t, r, "README.md")
	commit(t, r, "second")
	out := lessons.Check(l, r, 0)
	assert.True(t, out.Done)
	assert.Equal(t, []int{1, 2, 3}, out.Completed)
}

func TestCheckResumesFromTheSavedStep(t *testing.T) {
	l, r := start(t, "first-commit")
	add(t, r, "README.md")
	out := expect(t, l, r, 0, 2)
	assert.Equal(t, []int{1}, out.Completed)
	// Starting at step 2 does not look at step 1 again.
	out = expect(t, l, r, 1, 2)
	assert.Empty(t, out.Completed)
}

func TestBranchingLesson(t *testing.T) {
	l, r := start(t, "branching")
	p := track(t, l, r)
	p.at(1)
	sw(t, r, "feature", true)
	p.at(2)
	write(t, r, "notes.txt", "start\nidea\n")
	add(t, r, "notes.txt")
	commit(t, r, "idea")
	p.at(3)
	sw(t, r, "main", false)
	p.at(0)
}

func TestMergeTypesLesson(t *testing.T) {
	l, r := start(t, "fast-forward-vs-three-way")
	p := track(t, l, r)
	p.at(1)
	_, err := porcelain.Merge(r, "ahead", porcelain.MergeOptions{})
	do(t, err)
	p.at(2)
	_, err = porcelain.Merge(r, "diverged", porcelain.MergeOptions{})
	do(t, err)
	p.at(0)
}

func TestResolveConflictLesson(t *testing.T) {
	l, r := start(t, "resolve-conflict")
	expect(t, l, r, 0, 1)
	res, err := porcelain.Merge(r, "topic", porcelain.MergeOptions{})
	do(t, err)
	require.Equal(t, porcelain.MergeConflicted, res.Outcome)
	expect(t, l, r, 0, 2)

	write(t, r, "story.txt", "once upon a time\nthe hero sailed home\nthe end\n")
	expect(t, l, r, 0, 2)
	add(t, r, "story.txt")
	expect(t, l, r, 0, 3)
	commit(t, r, "")
	assert.True(t, lessons.Check(l, r, 0).Done)
}

func TestResolveConflictNeedsMarkersGone(t *testing.T) {
	l, r := start(t, "resolve-conflict")
	_, err := porcelain.Merge(r, "topic", porcelain.MergeOptions{})
	do(t, err)
	out := lessons.Check(l, r, 0)
	assert.Equal(t, 2, out.Next)
	assert.Contains(t, out.Missing, "conflict markers")

	write(t, r, "story.txt", "fixed\n")
	out = lessons.Check(l, r, 0)
	assert.Equal(t, 2, out.Next)
	assert.Contains(t, out.Missing, "not staged")
}

func TestUndoMistakesLesson(t *testing.T) {
	l, r := start(t, "undo-mistakes")
	expect(t, l, r, 0, 1)
	_, err := porcelain.Reset(r, "HEAD~1", porcelain.ResetHard)
	do(t, err)
	expect(t, l, r, 0, 2)
	_, err = porcelain.Revert(r, "HEAD", porcelain.PickOptions{})
	do(t, err)
	assert.True(t, lessons.Check(l, r, 0).Done)
}

func TestReflogRecoveryLesson(t *testing.T) {
	l, r := start(t, "reflog-recovery")
	nodes, err := porcelain.Log(r, porcelain.LogOptions{})
	do(t, err)
	assert.Len(t, nodes, 1, "the setup lost two commits")
	expect(t, l, r, 0, 1)

	_, err = porcelain.Reset(r, "HEAD@{1}", porcelain.ResetHard)
	do(t, err)
	expect(t, l, r, 0, 2)
	_, err = porcelain.CreateBranch(r, "rescued", "")
	do(t, err)
	assert.True(t, lessons.Check(l, r, 0).Done)
}

func TestPrepareStartsFromAFreshWorkspace(t *testing.T) {
	l, err := lessons.Find("first-commit")
	require.NoError(t, err)
	store := lessons.Store{Home: t.TempDir()}
	r, err := lessons.Prepare(store, l)
	do(t, err)
	write(t, r, "leftover.txt", "x")

	r, err = lessons.Prepare(store, l)
	do(t, err)
	assert.False(t, r.Work.Exists("leftover.txt"))
	again, err := lessons.Open(store, "first-commit")
	do(t, err)
	assert.Equal(t, r.Root, again.Root)

	_, err = lessons.Open(store, "branching")
	assert.ErrorContains(t, err, "lesson start branching")
}

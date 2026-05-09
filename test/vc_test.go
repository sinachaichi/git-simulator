package main

import (
	"testing"
	"vc/commands"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	assert.NotNil(t, vc)
}

func TestStatusUnmodifiedAtFirst(t *testing.T) {
	status := vc.Status()
	assert.Len(t, status.ModifiedFiles, 0)
	assert.Len(t, status.StagedFiles, 0)
}
func TestStatusModifiedAtFirst(t *testing.T) {
	status := vc.Status()
	assert.Len(t, status.ModifiedFiles, 0)
	assert.Len(t, status.StagedFiles, 0)
	// there is no commit yet
}

func TestStatusModified(t *testing.T) {
	vc = commands.Init(wd.Clone())
	vc.AddAll()
	vc.Commit("initial commit")

	vc.GetWorkDir().AppendToFile("README.md", "\nhello world")

	status := vc.Status()
	assert.ElementsMatch(t, []string{"README.md"}, status.ModifiedFiles)
	assert.Len(t, status.StagedFiles, 0)
}

func TestStatusStaged(t *testing.T) {
	vc = commands.Init(wd.Clone())
	vc.AddAll()
	vc.Commit("initial commit")

	vc.GetWorkDir().AppendToFile("README.md", "\nhello world")
	vc.Add("README.md")

	status := vc.Status()
	assert.Len(t, status.ModifiedFiles, 0)
	assert.ElementsMatch(t, []string{"README.md"}, status.StagedFiles)
}

func TestStatusMixed1(t *testing.T) {
	vc = commands.Init(wd.Clone())
	vc.AddAll()
	vc.Commit("initial commit")

	vc.GetWorkDir().CreateFile(".gitignore")
	vc.GetWorkDir().WriteToFile(".gitignore", "*.class")

	vc.GetWorkDir().AppendToFile("README.md", "\nhello world")
	vc.Add("README.md")

	status := vc.Status()
	assert.ElementsMatch(t, []string{".gitignore"}, status.ModifiedFiles)
	assert.ElementsMatch(t, []string{"README.md"}, status.StagedFiles)
}

func TestStatusMixed2(t *testing.T) {
	vc = commands.Init(wd.Clone())
	vc.AddAll()
	vc.Commit("initial commit")

	vc.GetWorkDir().CreateFile(".gitignore")
	vc.GetWorkDir().WriteToFile(".gitignore", "*.class")

	vc.GetWorkDir().AppendToFile("README.md", "\nhello world")
	vc.Add("README.md")

	vc.GetWorkDir().AppendToFile("README.md", "\nit's time to goodbye")

	status := vc.Status()
	assert.ElementsMatch(t, []string{".gitignore", "README.md"}, status.ModifiedFiles)
	assert.ElementsMatch(t, []string{"README.md"}, status.StagedFiles)
}

func TestLog1(t *testing.T) {
	vc = commands.Init(wd.Clone())
	vc.AddAll()
	vc.Commit("commit1")
	vc.Commit("commit2")
	vc.Commit("commit3")
	vc.Commit("commit4")

	assert.Equal(t,
		[]string{"commit4", "commit3", "commit2", "commit1"}, // in reverse order
		vc.Log(),
	)
}

func TestLog2(t *testing.T) {
	vc = commands.Init(wd.Clone())
	assert.Equal(t,
		[]string{},
		vc.Log(),
	)
}

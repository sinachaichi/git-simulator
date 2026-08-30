package friendly_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/friendly"
	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

func TestEveryKnownErrorGetsAHint(t *testing.T) {
	errs := []error{
		repo.ErrNotARepo, repo.ErrInsideGit, repo.ErrAlreadyExists, repo.ErrNoIdentity,
		porcelain.ErrNothingToCommit, porcelain.ErrEmptyMessage,
		&porcelain.PathspecError{Spec: "x"},
		&porcelain.LocalChangesError{Paths: []string{"a"}},
		&porcelain.LocalChangesError{Paths: []string{"a"}, Untracked: true},
		&index.UnmergedError{Paths: []string{"a"}}, porcelain.ErrUnmergedIndex,
		porcelain.ErrMergeInProgress, porcelain.ErrPickInProgress,
		porcelain.ErrNoMergeInProgress, porcelain.ErrNoPick,
		porcelain.ErrBranchExists, porcelain.ErrBranchNotFound, porcelain.ErrCurrentBranch, porcelain.ErrNotMerged,
		porcelain.ErrTagExists, porcelain.ErrTagNotFound,
		porcelain.ErrUnrelated, porcelain.ErrNotFastForward, porcelain.ErrNothingToMerge,
		porcelain.ErrNoStash, porcelain.ErrNoLocalChanges, porcelain.ErrEmptyPick, porcelain.ErrMergeCommit,
		&porcelain.RmError{Paths: []string{"a"}, Reason: "x"},
		&revparse.NotFoundError{Expr: "x"},
		&odb.AmbiguousError{Prefix: "abcd"},
	}
	for _, err := range errs {
		h, ok := friendly.For(err)
		assert.True(t, ok, "%T %v", err, err)
		assert.NotEmpty(t, h.Why, "%v", err)
		assert.NotEmpty(t, h.Try, "%v", err)
	}
}

func TestWrappedErrorsAreFound(t *testing.T) {
	err := fmt.Errorf("branch %q: %w", "x", porcelain.ErrBranchExists)
	h, ok := friendly.For(err)
	assert.True(t, ok)
	assert.Contains(t, h.Try, "branch")
}

func TestUntrackedAndModifiedFilesGetDifferentAdvice(t *testing.T) {
	a, _ := friendly.For(&porcelain.LocalChangesError{Paths: []string{"a"}})
	b, _ := friendly.For(&porcelain.LocalChangesError{Paths: []string{"a"}, Untracked: true})
	assert.NotEqual(t, a, b)
	assert.Contains(t, a.Try, "stash")
}

func TestUnknownErrorsHaveNoHint(t *testing.T) {
	_, ok := friendly.For(errors.New("disk on fire"))
	assert.False(t, ok)
	_, ok = friendly.For(nil)
	assert.False(t, ok)
}

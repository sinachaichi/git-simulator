package porcelain_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/merge"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func numbered(n int) string {
	out := ""
	for i := 1; i <= n; i++ {
		out += "line " + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "\n"
	}
	return out
}

// diverged builds main and feature from a common commit "base".
// Both add one commit. Files: shared.txt (10 lines), main-only, feature-only.
func diverged(t *testing.T, mainEdit, featureEdit func(*repo.Repo)) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "shared.txt", numbered(10))
	commitAll(t, r, "base")
	_, err := porcelain.CreateBranch(r, "feature", "")
	require.NoError(t, err)
	mainEdit(r)
	commitAll(t, r, "main work")
	_, err = porcelain.Switch(r, "feature", porcelain.SwitchOptions{})
	require.NoError(t, err)
	featureEdit(r)
	commitAll(t, r, "feature work")
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	return r
}

func disjointEdits(t *testing.T) *repo.Repo {
	return diverged(t,
		func(r *repo.Repo) { write(t, r, "main-only", "m\n") },
		func(r *repo.Repo) { write(t, r, "feature-only", "f\n") })
}

func commitOf(t *testing.T, r *repo.Repo, id objects.ID) *objects.Commit {
	t.Helper()
	c, err := r.ODB.ReadCommit(id)
	require.NoError(t, err)
	return c
}

func TestMergeAlreadyUpToDate(t *testing.T) {
	r := disjointEdits(t)
	_, err := porcelain.CreateBranch(r, "old", "HEAD~1")
	require.NoError(t, err)
	res, err := porcelain.Merge(r, "old", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeUpToDate, res.Outcome)
}

func TestMergeFastForward(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	_, err := porcelain.Switch(r, "feature", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	write(t, r, "a", "2\n")
	write(t, r, "b", "new\n")
	commitAll(t, r, "two")
	featureTip := headID(t, r)
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)

	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeFastForward, res.Outcome)
	assert.Equal(t, featureTip, headID(t, r))
	assert.Equal(t, "2\n", read(t, r, "a"))
	assert.Equal(t, "new\n", read(t, r, "b"))
	assert.True(t, status(t, r).Clean())

	log, _ := r.Refs.ReadLog("refs/heads/main")
	assert.Equal(t, "merge feature: Fast-forward", log[0].Message)
	assert.False(t, porcelain.MergeInProgress(r))
}

func TestMergeNoFFCreatesMergeCommit(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	_, err := porcelain.Switch(r, "feature", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	write(t, r, "b", "new\n")
	commitAll(t, r, "two")
	featureTip := headID(t, r)
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	mainTip := headID(t, r)

	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{NoFF: true})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeCommitted, res.Outcome)
	c := commitOf(t, r, res.Commit)
	assert.Equal(t, []objects.ID{mainTip, featureTip}, c.Parents)
	assert.Equal(t, "Merge branch 'feature'\n", c.Message)
}

func TestMergeFFOnlyRefusesDivergedBranches(t *testing.T) {
	r := disjointEdits(t)
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{FFOnly: true})
	assert.True(t, errors.Is(err, porcelain.ErrNotFastForward))
}

func TestMergeThreeWayCleanCombinesBothSides(t *testing.T) {
	r := disjointEdits(t)
	mainTip, featureTip := headID(t, r), revparseCommit(t, r, "feature")

	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeCommitted, res.Outcome)
	assert.Equal(t, 1, res.Files)
	assert.Equal(t, revparseCommit(t, r, "HEAD~2"), res.Base)

	c := commitOf(t, r, res.Commit)
	assert.Equal(t, []objects.ID{mainTip, featureTip}, c.Parents)
	assert.Equal(t, "m\n", read(t, r, "main-only"))
	assert.Equal(t, "f\n", read(t, r, "feature-only"))
	assert.True(t, status(t, r).Clean())

	for _, name := range []string{"HEAD", "refs/heads/main"} {
		log, _ := r.Refs.ReadLog(name)
		assert.Equal(t, "merge feature: Merge made by the 'ort' strategy.", log[0].Message)
	}
}

func TestMergeCombinesEditsInOneFile(t *testing.T) {
	r := diverged(t,
		func(r *repo.Repo) { write(t, r, "shared.txt", "top\n"+numbered(10)) },
		func(r *repo.Repo) { write(t, r, "shared.txt", numbered(10)+"bottom\n") })
	tick(t)
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "top\n"+numbered(10)+"bottom\n", read(t, r, "shared.txt"))
}

func TestMergeMessageMentionsBranchWhenNotOnMain(t *testing.T) {
	r := disjointEdits(t)
	_, err := porcelain.Switch(r, "develop", porcelain.SwitchOptions{Create: true})
	require.NoError(t, err)
	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Merge branch 'feature' into develop\n", commitOf(t, r, res.Commit).Message)

	r2 := disjointEdits(t)
	_, err = porcelain.CreateTag(r2, "v1", "feature", "")
	require.NoError(t, err)
	tick(t)
	res, err = porcelain.Merge(r2, "v1", porcelain.MergeOptions{Message: "custom words"})
	require.NoError(t, err)
	assert.Equal(t, "custom words\n", commitOf(t, r2, res.Commit).Message)
}

func conflicting(t *testing.T) *repo.Repo {
	t.Helper()
	return diverged(t,
		func(r *repo.Repo) { write(t, r, "shared.txt", "line 01\nMAIN\n"+numbered(10)[14:]) },
		func(r *repo.Repo) { write(t, r, "shared.txt", "line 01\nFEATURE\n"+numbered(10)[14:]) })
}

func TestMergeConflictLeavesMergeState(t *testing.T) {
	r := conflicting(t)
	mainTip, featureTip := headID(t, r), revparseCommit(t, r, "feature")
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeConflicted, res.Outcome)
	assert.Equal(t, []porcelain.ConflictInfo{{Path: "shared.txt", Kind: merge.ContentConflict}}, res.Conflicts)

	assert.Equal(t, mainTip, headID(t, r), "HEAD does not move")
	assert.True(t, porcelain.MergeInProgress(r))
	data, err := os.ReadFile(filepath.Join(r.GitDir, "MERGE_HEAD"))
	require.NoError(t, err)
	assert.Equal(t, featureTip.String()+"\n", string(data))

	assert.Contains(t, read(t, r, "shared.txt"), "<<<<<<< HEAD\nMAIN\n=======\nFEATURE\n>>>>>>> feature\n")
	ix, _ := r.LoadIndex()
	st := ix.Stages("shared.txt")
	assert.NotNil(t, st[0])
	assert.NotNil(t, st[1])
	assert.NotNil(t, st[2])
	s := status(t, r)
	assert.Equal(t, []porcelain.Unmerged{{Path: "shared.txt", Kind: porcelain.BothModified}}, s.Unmerged)
}

func TestMergeConflictBlocksCommitUntilResolved(t *testing.T) {
	r := conflicting(t)
	featureTip, mainTip := revparseCommit(t, r, "feature"), headID(t, r)
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)

	_, err = porcelain.Commit(r, porcelain.CommitOptions{Message: "try"})
	assert.ErrorContains(t, err, "unmerged paths")

	_, err = porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrMergeInProgress))

	write(t, r, "shared.txt", numbered(10)+"resolved\n")
	_, err = porcelain.Add(r, []string{"shared.txt"})
	require.NoError(t, err)
	tick(t)
	res, err := porcelain.Commit(r, porcelain.CommitOptions{})
	require.NoError(t, err)

	c := commitOf(t, r, res.ID)
	assert.Equal(t, []objects.ID{mainTip, featureTip}, c.Parents)
	assert.Equal(t, "Merge branch 'feature'\n", c.Message, "the message comes from MERGE_MSG")
	assert.False(t, porcelain.MergeInProgress(r))
	assert.True(t, status(t, r).Clean())
	log, _ := r.Refs.ReadLog("HEAD")
	assert.Equal(t, "commit (merge): Merge branch 'feature'", log[0].Message)
}

func TestMergeModifyDeleteAndAddAddConflicts(t *testing.T) {
	r := diverged(t,
		func(r *repo.Repo) {
			require.NoError(t, r.Work.Remove("shared.txt"))
			write(t, r, "both-new", "main\n")
		},
		func(r *repo.Repo) {
			write(t, r, "shared.txt", numbered(10)+"edit\n")
			write(t, r, "both-new", "feature\n")
		})
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, []porcelain.ConflictInfo{
		{Path: "both-new", Kind: merge.AddAdd},
		{Path: "shared.txt", Kind: merge.ModifyDelete},
	}, res.Conflicts)

	kinds := map[string]porcelain.ConflictKind{}
	for _, u := range status(t, r).Unmerged {
		kinds[u.Path] = u.Kind
	}
	assert.Equal(t, porcelain.BothAdded, kinds["both-new"])
	assert.Equal(t, porcelain.DeletedByUs, kinds["shared.txt"])
	assert.Equal(t, numbered(10)+"edit\n", read(t, r, "shared.txt"), "the surviving edit is kept for the user")
}

func TestMergeNoCommitStopsBeforeCommitting(t *testing.T) {
	r := disjointEdits(t)
	mainTip := headID(t, r)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{NoCommit: true})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeStopped, res.Outcome)
	assert.Equal(t, mainTip, headID(t, r))
	assert.True(t, porcelain.MergeInProgress(r))
	assert.Len(t, status(t, r).Staged, 1)

	tick(t)
	done, err := porcelain.Commit(r, porcelain.CommitOptions{})
	require.NoError(t, err)
	assert.Len(t, commitOf(t, r, done.ID).Parents, 2)
}

func TestMergeUnrelatedHistories(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a", "1\n")
	commitAll(t, r, "one")
	// A root commit that shares nothing with main.
	tree, _, err := r.ODB.Write(&objects.Tree{})
	require.NoError(t, err)
	who, err := r.Identity("author")
	require.NoError(t, err)
	orphan, _, err := r.ODB.Write(&objects.Commit{Tree: tree, Author: who, Committer: who, Message: "orphan\n"})
	require.NoError(t, err)
	require.NoError(t, r.Refs.Update("refs/heads/other", orphan))

	_, err = porcelain.Merge(r, "other", porcelain.MergeOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrUnrelated))
}

func TestMergeRefusesStagedChanges(t *testing.T) {
	r := disjointEdits(t)
	write(t, r, "main-only", "staged edit\n")
	_, err := porcelain.Add(r, []string{"main-only"})
	require.NoError(t, err)
	_, err = porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	var lc *porcelain.LocalChangesError
	assert.True(t, errors.As(err, &lc))
	assert.False(t, porcelain.MergeInProgress(r))
}

func TestMergeKeepsLocalEditsToFilesItDoesNotTouch(t *testing.T) {
	r := diverged(t,
		func(r *repo.Repo) { write(t, r, "main-only", "m\n") },
		func(r *repo.Repo) { write(t, r, "shared.txt", numbered(10)+"feature line\n") })
	write(t, r, "main-only", "local edit\n")

	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	assert.Equal(t, porcelain.MergeCommitted, res.Outcome)
	assert.Equal(t, "local edit\n", read(t, r, "main-only"))
	assert.Equal(t, []porcelain.Change{{Path: "main-only", Kind: porcelain.Modified}}, status(t, r).Unstaged)
}

func TestMergeRefusesLocalEditsToFilesItChanges(t *testing.T) {
	r := diverged(t,
		func(r *repo.Repo) { write(t, r, "main-only", "m\n") },
		func(r *repo.Repo) { write(t, r, "shared.txt", numbered(10)+"feature line\n") })
	write(t, r, "shared.txt", "my own work\n")

	mainTip := headID(t, r)
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	var lc *porcelain.LocalChangesError
	require.True(t, errors.As(err, &lc))
	assert.Equal(t, []string{"shared.txt"}, lc.Paths)
	assert.Equal(t, mainTip, headID(t, r))
	assert.Equal(t, "my own work\n", read(t, r, "shared.txt"))
	assert.False(t, porcelain.MergeInProgress(r))
}

func TestMergeRefusesUntrackedFileInTheWay(t *testing.T) {
	r := diverged(t,
		func(r *repo.Repo) { write(t, r, "main-only", "m\n") },
		func(r *repo.Repo) { write(t, r, "feature-only", "f\n") })
	write(t, r, "feature-only", "untracked, same name\n")
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	var lc *porcelain.LocalChangesError
	require.True(t, errors.As(err, &lc))
	assert.True(t, lc.Untracked)
}

func TestMergeIntoDetachedHead(t *testing.T) {
	r := disjointEdits(t)
	require.NoError(t, r.Refs.SetHeadDetached(headID(t, r)))
	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	h, _ := r.Refs.Head()
	assert.True(t, h.Detached())
	assert.Equal(t, res.Commit, h.ID)
}

func TestMergeOnUnbornBranchAndUnknownRevision(t *testing.T) {
	_, err := porcelain.Merge(newRepo(t), "x", porcelain.MergeOptions{})
	assert.True(t, errors.Is(err, porcelain.ErrNothingToMerge))
	_, err = porcelain.Merge(twoCommits(t), "ghost", porcelain.MergeOptions{})
	assert.Error(t, err)
}

// Real git merges a copy of the same repository. The tree of its merge
// commit must be the one we computed.
func TestMergeTreeMatchesRealGit(t *testing.T) {
	testutil.RequireGit(t)
	r := diverged(t,
		func(r *repo.Repo) {
			write(t, r, "shared.txt", "top\n"+numbered(10))
			write(t, r, "main-only", "m\n")
		},
		func(r *repo.Repo) {
			write(t, r, "shared.txt", numbered(10)+"bottom\n")
			write(t, r, "dir/feature-only", "f\n")
		})
	copyDir := t.TempDir()
	testutil.CopyDir(t, r.Root, copyDir)

	tick(t)
	res, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)
	ours := commitOf(t, r, res.Commit)

	gitDir := filepath.Join(copyDir, ".hood")
	testutil.Git(t, copyDir, "", "--git-dir="+gitDir, "--work-tree="+copyDir, "merge", "--no-edit", "feature")
	theirs := testutil.Git(t, copyDir, "", "--git-dir="+gitDir, "rev-parse", "HEAD^{tree}")
	assert.Equal(t, ours.Tree.String()+"\n", theirs)
}

func TestRealGitSeesOurConflictState(t *testing.T) {
	testutil.RequireGit(t)
	r := conflicting(t)
	_, err := porcelain.Merge(r, "feature", porcelain.MergeOptions{})
	require.NoError(t, err)

	git := func(args ...string) string {
		return testutil.Git(t, r.Root, "", append([]string{"--git-dir=" + r.GitDir, "--work-tree=" + r.Root}, args...)...)
	}
	assert.Equal(t, "UU shared.txt\n", git("status", "--porcelain", "--untracked-files=no"))
	stages := git("ls-files", "--unmerged")
	assert.Contains(t, stages, " 1\tshared.txt")
	assert.Contains(t, stages, " 2\tshared.txt")
	assert.Contains(t, stages, " 3\tshared.txt")
	assert.Equal(t, headID(t, r).String()+"\n", git("rev-parse", "HEAD"))
	assert.Equal(t, revparseCommit(t, r, "feature").String()+"\n", git("rev-parse", "MERGE_HEAD"))
}

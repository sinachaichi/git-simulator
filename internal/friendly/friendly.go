// Package friendly explains errors in plain words and suggests a next
// command. The cli prints this under the error message.
package friendly

import (
	"errors"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// Help explains what went wrong and what to try next.
type Help struct {
	Why string
	Try string
}

// For returns help for err, or false when there is nothing to add.
func For(err error) (Help, bool) {
	var (
		pathspec   *porcelain.PathspecError
		localEdits *porcelain.LocalChangesError
		rmErr      *porcelain.RmError
		unmerged   *index.UnmergedError
		notFound   *revparse.NotFoundError
		ambiguous  *odb.AmbiguousError
	)
	switch {
	case errors.Is(err, repo.ErrNotARepo):
		return Help{"hood keeps its data in a .hood folder, and there is none here or above.", "init"}, true
	case errors.Is(err, repo.ErrInsideGit):
		return Help{"hood never touches real git repositories, so it will not start inside one.", "init <a-new-folder> outside of the real repository"}, true
	case errors.Is(err, repo.ErrAlreadyExists):
		return Help{"There is already a repository here.", "status"}, true
	case errors.Is(err, repo.ErrNoIdentity):
		return Help{"A commit records who made it, and no name or email is set.", `config user.name "Your Name", then config user.email you@example.com`}, true
	case errors.Is(err, porcelain.ErrNothingToCommit):
		return Help{"A commit stores what is staged, and the index is the same as HEAD.", "status to see what changed, then add <file>"}, true
	case errors.Is(err, porcelain.ErrEmptyMessage):
		return Help{"Every commit needs a message, and there is no editor in the simulator.", `commit -m "what you changed"`}, true
	case errors.As(err, &pathspec):
		return Help{"No file or directory with that name is in the working tree or the index.", "status to list the files, and check the spelling"}, true
	case errors.As(err, &localEdits):
		if localEdits.Untracked {
			return Help{"An untracked file has the same name as a file that is about to be written, and it would be lost.", "move or delete the file, or add and commit it first"}, true
		}
		return Help{"These files have changes that are not committed, and the operation would overwrite them.", "commit them, put them aside with stash, or drop them with reset --hard"}, true
	case errors.As(err, &unmerged), errors.Is(err, porcelain.ErrUnmergedIndex):
		return Help{"Some files still have conflicts, so the index cannot be turned into a commit yet.", "open the files, fix the <<<<<<< markers, run add <file> for each, then commit. Or give up with merge --abort"}, true
	case errors.Is(err, porcelain.ErrMergeInProgress):
		return Help{"A merge stopped with conflicts and is waiting for you.", "status to see the files, then commit once they are fixed, or merge --abort"}, true
	case errors.Is(err, porcelain.ErrPickInProgress):
		return Help{"A cherry-pick or revert stopped with conflicts and is waiting for you.", "fix the files and run commit, or use --abort on the same command"}, true
	case errors.Is(err, porcelain.ErrNoMergeInProgress), errors.Is(err, porcelain.ErrNoPick):
		return Help{"There is nothing to continue or abort right now.", "status"}, true
	case errors.Is(err, porcelain.ErrBranchExists):
		return Help{"A branch with that name is already there.", "branch to list them, or pick another name"}, true
	case errors.Is(err, porcelain.ErrBranchNotFound):
		return Help{"There is no branch with that name.", "branch to list them, or switch -c <name> to create one"}, true
	case errors.Is(err, porcelain.ErrCurrentBranch):
		return Help{"You cannot delete or rename the branch you are standing on.", "switch to another branch first"}, true
	case errors.Is(err, porcelain.ErrNotMerged):
		return Help{"The branch has commits that no other branch has, so deleting it would lose them.", "merge it first, or use branch -D to delete it anyway"}, true
	case errors.Is(err, porcelain.ErrTagExists):
		return Help{"A tag with that name is already there.", "tag -l to list them, or tag -d <name> to delete it"}, true
	case errors.Is(err, porcelain.ErrTagNotFound):
		return Help{"There is no tag with that name.", "tag -l"}, true
	case errors.Is(err, porcelain.ErrUnrelated):
		return Help{"The two branches share no commit, so there is nothing to merge them against.", "check the branch name with branch"}, true
	case errors.Is(err, porcelain.ErrNotFastForward):
		return Help{"The branches have both moved on, so a fast-forward is not possible.", "merge without --ff-only to create a merge commit"}, true
	case errors.Is(err, porcelain.ErrNothingToMerge):
		return Help{"The current branch has no commits yet.", "commit something first"}, true
	case errors.Is(err, porcelain.ErrNoStash):
		return Help{"The stash has no entry with that number.", "stash list"}, true
	case errors.Is(err, porcelain.ErrNoLocalChanges):
		return Help{"Stash saves changes to tracked files, and there are none.", "status"}, true
	case errors.Is(err, porcelain.ErrEmptyPick):
		return Help{"The change is already part of this branch.", "log to check, there is nothing left to apply"}, true
	case errors.Is(err, porcelain.ErrMergeCommit):
		return Help{"A merge commit has two parents, and we cannot tell which side to use.", "pick or revert the commits inside the merge instead"}, true
	case errors.As(err, &rmErr):
		return Help{"rm protects work that is not committed.", "commit or stash first, use --cached to keep the file, or -f to force"}, true
	case errors.As(err, &notFound):
		return Help{"The name is not a branch, tag, commit id or HEAD expression.", "log --oneline --all to see the commits, branch to list branches"}, true
	case errors.As(err, &ambiguous):
		return Help{"The short id matches more than one object.", "use more characters of the id"}, true
	}
	return Help{}, false
}

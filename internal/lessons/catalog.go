package lessons

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/history"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

func catalog() []Lesson {
	return []Lesson{
		firstCommit(), branching(), mergeTypes(), resolveConflict(), undoMistakes(), reflogRecovery(),
	}
}

// ---- helpers for setups and checks ----

func commitFile(r *repo.Repo, path, content, msg string) error {
	if err := r.Work.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if _, err := porcelain.Add(r, []string{path}); err != nil {
		return err
	}
	_, err := porcelain.Commit(r, porcelain.CommitOptions{Message: msg})
	return err
}

func switchTo(r *repo.Repo, branch string, create bool) error {
	_, err := porcelain.Switch(r, branch, porcelain.SwitchOptions{Create: create})
	return err
}

func branchTip(r *repo.Repo, name string) (objects.ID, bool) {
	id, err := r.Refs.Read(refs.BranchPrefix + name)
	return id, err == nil
}

// reachable returns the commits reachable from HEAD, newest first.
func reachable(r *repo.Repo) []history.Node {
	head, err := r.Refs.Head()
	if err != nil || head.Unborn() {
		return nil
	}
	nodes, err := history.Walk(r.ODB, []objects.ID{head.ID})
	if err != nil {
		return nil
	}
	return nodes
}

func hasSubject(nodes []history.Node, subject string) bool {
	for _, n := range nodes {
		if n.Commit.Subject() == subject {
			return true
		}
	}
	return false
}

func onBranch(r *repo.Repo, name string) bool {
	head, err := r.Refs.Head()
	return err == nil && head.Branch() == name
}

func fileHas(r *repo.Repo, path, text string) bool {
	data, err := r.Work.ReadFile(path)
	return err == nil && strings.Contains(string(data), text)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ---- the lessons ----

func firstCommit() Lesson {
	return Lesson{
		Name:    "first-commit",
		Title:   "Your first commit",
		Summary: "Stage a file, commit it, and see what git stores.",
		Setup: func(r *repo.Repo) error {
			return r.Work.WriteFile("README.md", []byte("# My project\n"), 0o644)
		},
		Steps: []Step{
			{
				Task: "There is a new file called README.md. Stage it with `add README.md`.",
				Hint: "Run `status` first. The file is listed as untracked.",
				Check: func(r *repo.Repo) (bool, string) {
					ix, err := r.LoadIndex()
					if err == nil {
						if _, ok := ix.Get("README.md"); ok {
							return true, ""
						}
					}
					return false, "README.md is not in the index yet. Try `add README.md`."
				},
			},
			{
				Task: "Save the staged file as a commit: `commit -m \"first commit\"`.",
				Hint: "Turn on X-ray with `xray on` to see the blob, tree and commit that get created.",
				Check: func(r *repo.Repo) (bool, string) {
					if len(reachable(r)) == 0 {
						return false, "There is no commit yet. Try `commit -m \"first commit\"`."
					}
					return true, ""
				},
			},
			{
				Task: "Change README.md (for example `append README.md more text`), stage it and commit again.",
				Hint: "A second commit points back to the first one. Look at it with `log`.",
				Check: func(r *repo.Repo) (bool, string) {
					if len(reachable(r)) < 2 {
						return false, "There is only one commit. Change the file, then `add` and `commit` again."
					}
					return true, ""
				},
			},
		},
	}
}

func branching() Lesson {
	return Lesson{
		Name:    "branching",
		Title:   "Branches are just pointers",
		Summary: "Create a branch, commit on it, and switch back.",
		Setup: func(r *repo.Repo) error {
			return commitFile(r, "notes.txt", "start\n", "add notes")
		},
		Steps: []Step{
			{
				Task: "Create a branch called feature and switch to it: `switch -c feature`.",
				Hint: "A branch is a file in .hood/refs/heads that holds a commit id. Look with `branch -v`.",
				Check: func(r *repo.Repo) (bool, string) {
					if onBranch(r, "feature") {
						return true, ""
					}
					return false, "HEAD is not on a branch called feature yet."
				},
			},
			{
				Task: "Commit a change on feature, such as a new line in notes.txt.",
				Hint: "`append notes.txt a new idea`, then `add notes.txt` and `commit -m \"idea\"`.",
				Check: func(r *repo.Repo) (bool, string) {
					feature, _ := branchTip(r, "feature")
					main, _ := branchTip(r, "main")
					if feature.IsZero() || feature == main {
						return false, "feature still points at the same commit as main. Make a commit on it."
					}
					return true, ""
				},
			},
			{
				Task: "Switch back to main with `switch main` and check that notes.txt has its old content.",
				Hint: "`cat notes.txt` before and after the switch shows what the working tree does.",
				Check: func(r *repo.Repo) (bool, string) {
					if !onBranch(r, "main") {
						return false, "HEAD is not on main yet."
					}
					return true, ""
				},
			},
		},
	}
}

func mergeTypes() Lesson {
	return Lesson{
		Name:    "fast-forward-vs-three-way",
		Title:   "Fast-forward and three-way merges",
		Summary: "See when a merge only moves a pointer and when it needs a new commit.",
		Setup: func(r *repo.Repo) error {
			if err := commitFile(r, "base.txt", "base\n", "base"); err != nil {
				return err
			}
			for _, b := range []string{"ahead", "diverged"} {
				if err := switchTo(r, b, true); err != nil {
					return err
				}
				if err := commitFile(r, b+".txt", b+"\n", "work on "+b); err != nil {
					return err
				}
				if err := switchTo(r, "main", false); err != nil {
					return err
				}
			}
			return nil
		},
		Steps: []Step{
			{
				Task: "Branch ahead has one commit on top of main. Merge it: `merge ahead`.",
				Hint: "Nothing new has to be created. Check with `graph` that main and ahead point at the same commit.",
				Check: func(r *repo.Repo) (bool, string) {
					main, _ := branchTip(r, "main")
					ahead, _ := branchTip(r, "ahead")
					if main != ahead {
						return false, "main has not moved to the commit of ahead yet."
					}
					return true, ""
				},
			},
			{
				Task: "Branch diverged started from the old main. Merge it: `merge diverged`.",
				Hint: "Both branches have commits the other does not, so git has to create a merge commit with two parents.",
				Check: func(r *repo.Repo) (bool, string) {
					head, err := r.Refs.Head()
					if err != nil || head.Unborn() {
						return false, "HEAD has no commit."
					}
					c, err := r.ODB.ReadCommit(head.ID)
					if err != nil || len(c.Parents) != 2 {
						return false, "The tip of main is not a merge commit yet. Try `merge diverged`."
					}
					return true, ""
				},
			},
		},
	}
}

func resolveConflict() Lesson {
	return Lesson{
		Name:    "resolve-conflict",
		Title:   "Resolve a merge conflict",
		Summary: "Two branches changed the same line. Fix it by hand.",
		Setup: func(r *repo.Repo) error {
			if err := commitFile(r, "story.txt", "once upon a time\nthe hero set out\nthe end\n", "story"); err != nil {
				return err
			}
			if err := switchTo(r, "topic", true); err != nil {
				return err
			}
			if err := commitFile(r, "story.txt", "once upon a time\nthe hero stayed home\nthe end\n", "hero stays"); err != nil {
				return err
			}
			if err := switchTo(r, "main", false); err != nil {
				return err
			}
			return commitFile(r, "story.txt", "once upon a time\nthe hero sailed away\nthe end\n", "hero sails")
		},
		Steps: []Step{
			{
				Task: "Merge topic into main: `merge topic`. It will stop with a conflict.",
				Hint: "`status` lists story.txt as `both modified`, and `cat story.txt` shows the markers.",
				Check: func(r *repo.Repo) (bool, string) {
					if porcelain.MergeInProgress(r) {
						return true, ""
					}
					if hasSubject(reachable(r), "hero stays") {
						return true, "" // already merged, later steps will check the result
					}
					return false, "The merge has not been started. Try `merge topic`."
				},
			},
			{
				Task: "Edit story.txt so that it has one version of the second line and no <<<<<<< markers, then `add story.txt`.",
				Hint: "`write story.txt <your text>` replaces the whole file. Keep all three lines.",
				Check: func(r *repo.Repo) (bool, string) {
					if fileHas(r, "story.txt", "<<<<<<<") || fileHas(r, "story.txt", ">>>>>>>") {
						return false, "story.txt still has conflict markers."
					}
					ix, err := r.LoadIndex()
					if err != nil || len(ix.Conflicts()) > 0 {
						return false, "The file is fixed but not staged. Run `add story.txt`."
					}
					return true, ""
				},
			},
			{
				Task: "Finish the merge with `commit`. The message is already prepared.",
				Hint: "The new commit has two parents. Check with `log --graph --oneline`.",
				Check: func(r *repo.Repo) (bool, string) {
					head, err := r.Refs.Head()
					if err != nil || head.Unborn() || porcelain.MergeInProgress(r) {
						return false, "The merge is not finished yet."
					}
					c, err := r.ODB.ReadCommit(head.ID)
					if err != nil || len(c.Parents) != 2 {
						return false, "The tip of main is not a merge commit yet."
					}
					return true, ""
				},
			},
		},
	}
}

func undoMistakes() Lesson {
	return Lesson{
		Name:    "undo-mistakes",
		Title:   "Undo with reset and revert",
		Summary: "Throw away a private commit, and undo a shared one without rewriting history.",
		Setup: func(r *repo.Repo) error {
			for _, c := range [][3]string{
				{"app.txt", "app\n", "add app"},
				{"bad.txt", "this breaks everything\n", "add bad change"},
				{"typo.txt", "tpyo\n", "local typo"},
			} {
				if err := commitFile(r, c[0], c[1], c[2]); err != nil {
					return err
				}
			}
			return nil
		},
		Steps: []Step{
			{
				Task: "The last commit, `local typo`, was never shared. Remove it from the branch with `reset --hard HEAD~1`.",
				Hint: "reset moves the branch pointer back. The commit is not deleted yet, `reflog` still shows it.",
				Check: func(r *repo.Repo) (bool, string) {
					if hasSubject(reachable(r), "local typo") {
						return false, "The commit `local typo` is still on the branch."
					}
					return true, ""
				},
			},
			{
				Task: "`add bad change` was already shared, so do not rewrite history. Undo it with `revert HEAD`.",
				Hint: "revert makes a new commit that does the opposite. Look at it with `show`.",
				Check: func(r *repo.Repo) (bool, string) {
					if !hasSubject(reachable(r), `Revert "add bad change"`) {
						return false, "There is no revert commit yet."
					}
					return true, ""
				},
			},
		},
	}
}

func reflogRecovery() Lesson {
	return Lesson{
		Name:    "reflog-recovery",
		Title:   "Recover lost commits with the reflog",
		Summary: "A hard reset made commits unreachable. Get them back.",
		Setup: func(r *repo.Repo) error {
			for _, c := range [][3]string{
				{"start.txt", "start\n", "start"},
				{"important.txt", "do not lose me\n", "important work"},
				{"more.txt", "more\n", "more work"},
			} {
				if err := commitFile(r, c[0], c[1], c[2]); err != nil {
					return err
				}
			}
			_, err := porcelain.Reset(r, "HEAD~2", porcelain.ResetHard)
			return err
		},
		Steps: []Step{
			{
				Task: "Oops: `important work` and `more work` are gone from `log`. Find them with `reflog` and bring them back with `reset --hard <id>` (or `HEAD@{1}`).",
				Hint: "The reflog lists every place HEAD has been. The line before `reset: moving to HEAD~2` is where you were.",
				Check: func(r *repo.Repo) (bool, string) {
					if hasSubject(reachable(r), "more work") {
						return true, ""
					}
					return false, "`more work` is still not reachable from HEAD."
				},
			},
			{
				Task: "Protect it with a branch: `branch rescued`.",
				Hint: "A commit that a branch points at can not become unreachable.",
				Check: func(r *repo.Repo) (bool, string) {
					rescued, ok := branchTip(r, "rescued")
					head, _ := r.Refs.Head()
					if !ok || rescued != head.ID {
						return false, "There is no branch called rescued at HEAD yet."
					}
					return true, ""
				},
			},
		},
	}
}

// Prepare creates the workspace repo for a lesson, running its setup.
// An existing workspace is removed first, so a lesson always starts clean.
func Prepare(s Store, l Lesson) (*repo.Repo, error) {
	dir := s.Workspace(l.Name)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	r, err := repo.Init(dir, repo.DefaultBranch)
	if err != nil {
		return nil, fmt.Errorf("lesson workspace: %w", err)
	}
	if err := r.ConfigSet("user.name", "Student"); err != nil {
		return nil, err
	}
	if err := r.ConfigSet("user.email", "student@hood.local"); err != nil {
		return nil, err
	}
	if err := l.Setup(r); err != nil {
		return nil, fmt.Errorf("setting up lesson %s: %w", l.Name, err)
	}
	return r, nil
}

// Open returns the repo of a lesson that was started before.
func Open(s Store, name string) (*repo.Repo, error) {
	dir := s.Workspace(name)
	if !exists(filepath.Join(dir, ".hood")) {
		return nil, fmt.Errorf("lesson %s has no workspace yet, run `lesson start %s`", name, name)
	}
	return repo.Open(dir)
}

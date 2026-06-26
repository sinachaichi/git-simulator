// Package repo ties the object database, refs, index and working tree of
// one simulated repository together.
package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/odb"
	"github.com/sinachaichi/git-simulator/internal/refs"
	"github.com/sinachaichi/git-simulator/internal/workdir"
)

// DefaultBranch is the branch a new repo starts on.
const DefaultBranch = "main"

// Errors callers can match with errors.Is.
var (
	ErrNotARepo      = errors.New("not a hood repository")
	ErrAlreadyExists = errors.New("repository already exists")
	ErrInsideGit     = errors.New("directory is inside a real git repository")
)

// Repo is one simulated repository on disk.
type Repo struct {
	Root   string
	GitDir string
	Work   *workdir.WorkDir
	ODB    *odb.Store
	Refs   *refs.Store
}

// Init creates a repository in dir. It refuses to run inside a real git
// working tree, because a simulator must never touch real repositories.
func Init(dir, branch string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	if gitRoot := findUp(abs, ".git"); gitRoot != "" {
		return nil, fmt.Errorf("%w: %s has a .git directory", ErrInsideGit, gitRoot)
	}
	gitDir := filepath.Join(abs, workdir.MetaDir)
	if _, err := os.Stat(gitDir); err == nil {
		return nil, fmt.Errorf("%w in %s", ErrAlreadyExists, abs)
	}
	for _, sub := range []string{"objects", "refs/heads", "refs/tags"} {
		if err := os.MkdirAll(filepath.Join(gitDir, filepath.FromSlash(sub)), 0o755); err != nil {
			return nil, err
		}
	}
	// A real git can open this directory with --git-dir, which the
	// differential tests rely on.
	cfg := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		return nil, err
	}
	r, err := Open(abs)
	if err != nil {
		return nil, err
	}
	if err := r.Refs.SetHeadBranch(branch); err != nil {
		return nil, err
	}
	return r, nil
}

// Open opens the repository whose working tree is exactly dir.
func Open(dir string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	gitDir := filepath.Join(abs, workdir.MetaDir)
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: no %s directory in %s", ErrNotARepo, workdir.MetaDir, abs)
	}
	work, err := workdir.New(abs)
	if err != nil {
		return nil, err
	}
	store, err := odb.New(filepath.Join(gitDir, "objects"))
	if err != nil {
		return nil, err
	}
	return &Repo{Root: abs, GitDir: gitDir, Work: work, ODB: store, Refs: &refs.Store{Dir: gitDir}}, nil
}

// Discover opens the repository that contains start, looking in parent
// directories like git does.
func Discover(start string) (*Repo, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	root := findUp(abs, workdir.MetaDir)
	if root == "" {
		return nil, fmt.Errorf("%w: no %s directory in %s or any parent", ErrNotARepo, workdir.MetaDir, abs)
	}
	return Open(root)
}

// findUp returns the closest directory at or above start that contains name.
func findUp(start, name string) string {
	for dir := start; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return dir
		} else if !errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// IndexPath is where the staging area is stored.
func (r *Repo) IndexPath() string { return filepath.Join(r.GitDir, "index") }

// LoadIndex reads the staging area.
func (r *Repo) LoadIndex() (*index.Index, error) { return index.Load(r.IndexPath()) }

// SaveIndex writes the staging area.
func (r *Repo) SaveIndex(ix *index.Index) error { return ix.Save(r.IndexPath()) }

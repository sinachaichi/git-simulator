package commands

import (
	"crypto/sha1"
	"fmt"
	"vc/workdir"
	"os"
	"io"	
	"path/filepath"
)

type Commit struct {
    Message  string
    FileSnap map[string]string
}

type VC struct {
	WorkDir     *workdir.WorkDir
	StagedFiles map[string]bool
	ModifiedFiles map[string]bool
	Commits     []Commit
	FileHashes  map[string]string 
}

type StatusResult struct {
	ModifiedFiles []string
	StagedFiles   []string
}


func hashFile(path string) (string, error) {
    file, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer file.Close()

    hasher := sha1.New()
    if _, err := io.Copy(hasher, file); err != nil {
        return "", err
    }

    return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func Init(wd *workdir.WorkDir) *VC {
	vc:= &VC{
		WorkDir:     wd,
		StagedFiles: make(map[string]bool),
		ModifiedFiles: make(map[string]bool),
		Commits:     []Commit{},
		FileHashes:  make(map[string]string),
	}
	vc.computeInitialHashes()
	vc.computeInitialStages()
	return vc
}

func (vc *VC) computeInitialHashes() {
	err := filepath.Walk(vc.WorkDir.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(vc.WorkDir.Path, path)
			if err != nil {
				return err
			}
			hash, err := hashFile(path)
			if err != nil {
				return err
			}
			vc.FileHashes[relPath] = hash
		}
		return nil
	})

	if err != nil {
		panic(err)
	}
}

func (vc *VC) computeInitialStages() {
	err := filepath.Walk(vc.WorkDir.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(vc.WorkDir.Path, path)
			if err != nil {
				return err
			}
			vc.StagedFiles[relPath] = false
		}
		return nil
	})

	if err != nil {
		panic(err)
	}
}


func (vc *VC) GetWorkDir() *workdir.WorkDir {
	return vc.WorkDir
}

func (vc *VC) AddAll() {
	root := vc.WorkDir.Path
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() { 
            relativePath, err := filepath.Rel(root, path)
            if err != nil {
                return err
            }
            vc.StagedFiles[relativePath] = true
			vc.ModifiedFiles[relativePath] = false
			vc.FileHashes[relativePath], _ = hashFile(path)
        }
        return nil
    })

    if err != nil {
        panic(err)
    }
}

func (vc *VC) Commit(message string) {
	commitSnapshot := make(map[string]string)
    for file, isStaged := range vc.StagedFiles {
        if isStaged {
            content, err := os.ReadFile(filepath.Join(vc.WorkDir.Path, file))
            if err != nil {
                fmt.Printf("Error reading file %s: %v\n", file, err)
                continue
            }
            commitSnapshot[file] = string(content)
        }
        vc.StagedFiles[file] = false
    }

    newCommit := Commit{
        Message:  message,
        FileSnap: commitSnapshot,
    }
    vc.Commits = append(vc.Commits, newCommit)
}

func (vc *VC) Add(files ...string) {
	root := vc.WorkDir.Path
	for _, file := range files {
		vc.StagedFiles[file] = true
		vc.ModifiedFiles[file] = false
		vc.FileHashes[file], _ = hashFile(file)
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() { 
				relativePath, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				if relativePath == file {
					vc.FileHashes[relativePath], _ = hashFile(path)
				}
			}
			return nil
		})

		if err != nil {
			panic(err)
		}
	}
}

func (vc *VC) Status() *StatusResult {
	modified := []string{}
    staged := []string{}
    err := filepath.Walk(vc.WorkDir.Path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            relPath, err := filepath.Rel(vc.WorkDir.Path, path)
            if err != nil {
                return err
            }
            currentHash, err := hashFile(path)
            if err != nil {
                return err
            }

			stagedState := vc.StagedFiles[relPath]
			lastHash := vc.FileHashes[relPath]
			
			if lastHash != currentHash {
				vc.ModifiedFiles[relPath] = true
				modified = append(modified, relPath)
			}

			if stagedState {
				staged = append(staged, relPath)
				}
            
        }
        return nil
    })

    if err != nil {
        panic(err)
    }

    return &StatusResult{
        ModifiedFiles: modified,
        StagedFiles:   staged,
    }
}

func (vc *VC) Log() []string {
	commitsMessages := []string{}
	for _, commit := range vc.Commits {
		commitsMessages = append(commitsMessages, commit.Message)
	}
	for i, j := 0, len(commitsMessages)-1; i < j; i, j = i+1, j-1 {
        commitsMessages[i], commitsMessages[j] = commitsMessages[j], commitsMessages[i]
    }
	return commitsMessages
}

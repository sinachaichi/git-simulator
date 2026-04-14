package workdir

import (
	cp "github.com/otiai10/copy"
	"os"
	"path/filepath"
)

type WorkDir struct {
	Path string
}

func InitEmptyWorkDir() *WorkDir {
	baseDir := "../test_workdir"
    baseErr := os.MkdirAll(baseDir, 0755)
    if baseErr != nil {
        panic(baseErr)
    }
	path, err := os.MkdirTemp(baseDir, "wowkdir")
	if err != nil {
		panic(err)
	}
	absoluteRoot, _ := filepath.Abs(path)
	return &WorkDir{Path: absoluteRoot}
}

func (wd *WorkDir) CreateFile(filename string) error {
	file, err := os.Create(filepath.Join(wd.Path, filename))
	if err != nil {
		return err
	}
	return file.Close()
}

func (wd *WorkDir) CreateDir(dirname string) error {
	return os.Mkdir(filepath.Join(wd.Path, dirname), 0755)
}

func (wd *WorkDir) WriteToFile(filename, content string) error {
	f, err := os.OpenFile(filepath.Join(wd.Path, filename), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return os.WriteFile(filepath.Join(wd.Path, filename), []byte(content), 0644)
}

func (wd *WorkDir) AppendToFile(filename, content string) error {
	f, err := os.OpenFile(filepath.Join(wd.Path, filename), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func (wd *WorkDir) ListFilesRoot() []string {
	var files []string
	absoluteRoot, _ := filepath.Abs(wd.Path)
	err := filepath.Walk(wd.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(absoluteRoot, path)
			if err != nil {
				return err
			}
			files = append(files, relativePath)
		}
		return nil
	})

	if err != nil {
		return []string{}
	}
	return files
}

func (wd *WorkDir) ListFilesIn(dir string) ([]string, error) {
	var files []string
	path := filepath.Join(wd.Path, dir)
	absoluteRoot, absError := filepath.Abs(path)
	if absError != nil {
		return []string{}, absError
	}

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(absoluteRoot, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.Join(dir, relativePath))
		}
		return nil
	})

	if err != nil {
		return []string{}, err
	}
	return files, nil
}

func (wd *WorkDir) CatFile(filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join(wd.Path, filename))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (wd *WorkDir) Clone() *WorkDir {
	baseDir := "../test_workdir"
    baseErr := os.MkdirAll(baseDir, 0755)
    if baseErr != nil {
        panic(baseErr)
    }
	clonePath, err := os.MkdirTemp(baseDir, "workdirClone")
	if err != nil {
		panic(err)
	}
	absoluteRoot, _ := filepath.Abs(clonePath)
	copyErr := cp.Copy(wd.Path, absoluteRoot)

	if copyErr != nil {
		panic(copyErr)
	}
	return &WorkDir{Path: absoluteRoot}
}

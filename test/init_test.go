package main

import (
	"os"
	"testing"
	workdir "vc/workdir"
)

var wd *workdir.WorkDir

func TestMain(m *testing.M) {
	Setup()
	os.Exit(m.Run())
}

func Setup() {
	wd = workdir.InitEmptyWorkDir()

	err := wd.CreateFile("README.md")
	if err != nil {
		panic(err)
	}
	err = wd.CreateDir("src")
	if err != nil {
		panic(err)
	}
	err = wd.CreateFile("src/main.go")
	if err != nil {
		panic(err)
	}

	err = wd.CreateDir("src/workdir")
	if err != nil {
		panic(err)
	}
	err = wd.CreateFile("src/workdir/file1.go")
	if err != nil {
		panic(err)
	}
	err = wd.CreateFile("src/workdir/file2.go")
	if err != nil {
		panic(err)
	}

	err = wd.WriteToFile("README.md", "### MY GIT IMPL")
	if err != nil {
		panic(err)
	}

}

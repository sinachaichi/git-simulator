// Command hood is a tiny git simulator that shows what happens under the
// hood after every command.
package main

import (
	"os"

	"github.com/sinachaichi/git-simulator/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	env := cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(cli.Run(os.Args[1:], env, version))
}

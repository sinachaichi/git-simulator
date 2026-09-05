// Command hood is a tiny git simulator that shows what happens under the
// hood after every command.
package main

import (
	"os"

	"github.com/sinachaichi/git-simulator/internal/cli"
	"github.com/sinachaichi/git-simulator/internal/repl"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	env := cli.Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	// `hood` and `hood shell` open the interactive shell. Everything else
	// is a one-shot command.
	if len(os.Args) == 1 || (len(os.Args) == 2 && os.Args[1] == "shell") {
		os.Exit(repl.Run(env, version))
	}
	os.Exit(cli.Run(os.Args[1:], env, version))
}

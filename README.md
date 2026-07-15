# git-simulator

A small git written in Go, for learning how git works inside. The binary
is called `hood`. It keeps its own repositories in a `.hood` folder, uses
the same object format as git, and never calls real git or touches a real
repository.

I started this as a toy that copied files around on every commit. It now
stores real blobs, trees and commits, so the ids you see are the ones git
would compute for the same content, author and date.

## Install

```
go install github.com/sinachaichi/git-simulator/cmd/hood@latest
```

Or clone it and run `make build`, which writes `bin/hood`.

## Try it

```
mkdir demo && cd demo
hood init
hood config user.name "Ada Lovelace"
hood config user.email ada@example.com
echo "hello" > hello.txt
hood add hello.txt
hood commit -m "first commit"
hood log
hood cat-file -p HEAD
```

`hood --help` lists the commands.

## What works now

`init`, `add`, `commit` (with `-a`, `--amend`), `status`, `log`
(`--oneline`, `-n`, `--all`), `cat-file`, `hash-object`, `ls-files`,
`config`.

Branches, diff, merge, reset, stash and the rest are not there yet.

## Tests

```
make test
```

Some tests compare our output with real git when it is installed. They
are skipped when it is not.

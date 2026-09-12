# git-simulator

A small git written in Go, for learning how git works inside. The binary
is called `hood`, as in "under the hood". After each command it can show
you what happened internally: which objects were written, which refs
moved, what changed in the index.

It does not call real git and it never touches a real repository. It
refuses to start inside a folder that has a `.git`.

I started this as a toy that copied files around on every commit. It is now
a real (if small) implementation of the object model: blobs, trees,
commits and tags are hashed with SHA-1 and stored as zlib files in the same
layout git uses. For the same content, author and date you get the same
ids as real git. The tests check this against real git when it is
installed.

![A short session: add, commit, branch, merge and graph, with X-ray on](docs/demo.gif)

The same session as an asciicast is in [docs/demo.cast](docs/demo.cast).
You can play it with `asciinema play docs/demo.cast`.

## Install

```
go install github.com/sinachaichi/git-simulator/cmd/hood@latest
```

You need Go 1.22 or newer. To build from a clone instead:

```
git clone https://github.com/sinachaichi/git-simulator
cd git-simulator
make build        # writes bin/hood
```

## Quick start

Run `hood` with no arguments to get the interactive shell. X-ray is on by
default there.

```
$ hood
hood> init
hood> config user.name "Ada Lovelace"
hood> config user.email ada@example.com
hood> write hello.txt hello
hood> add hello.txt
hood> commit -m "first commit"
hood (main)> graph
```

There is no editor in the shell, so it has a few helpers: `write`,
`append`, `cat`, `ls`, `del`, `cd`. Type `help` to see them all.

You can also run single commands like a normal tool:

```
hood init
hood --xray commit -m "first commit"
hood log --graph --oneline --all
```

`--xray` or `HOOD_XRAY=1` turns the summary on for one-shot commands.

## What X-ray shows

```
$ hood --xray commit -m "first commit"
[main (root-commit) 83eef82] first commit
x-ray
  objects
    + tree   0d8a474  29 bytes
    + commit 83eef82  164 bytes
  refs
    refs/heads/main  0000000 -> 83eef82
    reflog refs/heads/main: commit (initial): first commit
    reflog HEAD: commit (initial): first commit
```

`+` is a new object, `=` is one that was already stored (adding the same
content twice writes nothing new). The index, the working tree, HEAD and
state files like `MERGE_HEAD` have their own sections.

## Lessons

Guided exercises that check the real state of a repository, not what you
typed.

```
hood lesson list
hood lesson start resolve-conflict
hood lesson check
```

There are six: `first-commit`, `branching`, `fast-forward-vs-three-way`,
`resolve-conflict`, `undo-mistakes` and `reflog-recovery`. `lesson hint`
gives a nudge and `lesson reset` starts over. Progress is saved in
`progress.json` under `$HOOD_HOME`, which defaults to a `hood` folder in
your user config directory.

## Look at it with real git

A simulated repository has the same layout as a real one, with `.hood`
instead of `.git`. So you can point real git at it and compare:

```
git --git-dir=.hood log --graph --oneline
git --git-dir=.hood cat-file -p HEAD
git --git-dir=.hood fsck --strict
```

That is how most of the tests work.

## Commands

| Command | Works | Notes |
|---|---|---|
| `init` | yes | `-b <branch>` |
| `add` | yes | files, directories, `.` and `-A`. No `-p`, no ignore files |
| `rm` | yes | `--cached`, `-f`, `-r`, with git's safety checks |
| `status` | yes | long format only |
| `commit` | yes | `-m` (repeatable), `-a`, `--amend`, `--allow-empty`. No editor, no signing |
| `log` | yes | `--oneline`, `--graph`, `--all`, `-n`, revisions. No `--format` or `--stat` |
| `graph` | yes | shorthand for `log --graph --oneline --all` |
| `diff` | yes | working tree, `--staged`, revisions, paths, `-U`, `--name-only`, `--name-status`. No `--stat`, no rename detection |
| `show` | yes | commits, tags, trees, blobs. Merge commits show no patch |
| `branch` | yes | list, create, `-d`, `-D`, `-m`, `-v` |
| `switch`, `checkout` | yes | branches, `-c` / `-b`, `--detach`, revisions. No `checkout -- <file>` |
| `merge` | yes | fast-forward, `--no-ff`, three-way, `--no-commit`, `--abort`, conflict markers |
| `reset` | yes | `--soft`, `--mixed`, `--hard`, and paths |
| `revert` | yes | one commit, `-n`, `--continue`, `--abort` |
| `cherry-pick` | yes | one commit, `-n`, `--continue`, `--abort` |
| `stash` | yes | push, list, show, apply, pop, drop. Untracked files are not stashed |
| `tag` | yes | lightweight and annotated, `-l`, `-d` |
| `reflog` | yes | HEAD, branches, stash |
| `cat-file` | yes | `-t`, `-s`, `-p` |
| `hash-object`, `ls-files`, `config` | yes | the basics |
| `lesson` | yes | `list`, `start`, `check`, `hint`, `status`, `reset` |

Not implemented: `rebase`, `bisect`, remotes (`clone`, `fetch`, `pull`,
`push`), ignore files, packfiles, hooks, submodules, worktrees, signing.

## Differences from git

Things that behave differently on purpose or because I have not done them:

- Merges do not detect renames. A renamed and edited file looks like a
  delete plus an add. When two branches have more than one best common
  ancestor, the newest one is used.
- `log --graph` matches git for the shapes I compared (linear, branch and
  merge, two tips, two merges in a row). When a lane has to cross other
  lanes before it joins, git sometimes draws one more connector line.
- Merging a file that is a directory on the other side is refused.
- Objects are always loose files. There are no packfiles.

## Development

```
make test    # go test ./...
make lint    # golangci-lint
make fmt     # gofumpt
make cover   # coverage summary
```

Tests that compare against real git skip themselves when `git` is not on
the PATH. See [ARCHITECTURE.md](ARCHITECTURE.md) for how the code is laid
out and [CONTRIBUTING.md](CONTRIBUTING.md) for how to send a change.

## License

MIT, see [LICENSE](LICENSE).

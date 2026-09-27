# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
uses [Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-09-26

First stable release.

### Added

- Interactive shell with a branch prompt, history, tab completion, X-ray on
  by default and helpers to edit files (`write`, `append`, `cat`, `ls`,
  `del`). `hood` with no arguments opens it.
- Six guided lessons with `lesson list`, `start`, `check`, `hint`, `status`
  and `reset`. Progress is saved in `progress.json`.
- X-ray mode (`--xray`, `HOOD_XRAY`): objects written, refs moved, index
  changes, files written and state files, after each command.
- `log --graph` and `graph`, with colored lanes, branch labels and a HEAD
  marker.
- Friendly errors: a `why` line and a `try` line under each known error.
- Release builds for linux, macOS and windows through goreleaser.
- Binary end-to-end tests, and differential tests against real git.

## [0.5.0] - 2026-08-16

### Added

- `merge` with fast-forward, `--no-ff`, three-way merges, conflict markers,
  `--no-commit` and `--abort`. Conflicted files get stages 1 to 3 in the
  index, and `commit` finishes the merge.
- `reset` (`--soft`, `--mixed`, `--hard` and paths), `revert`,
  `cherry-pick`, `stash` (push, list, show, apply, pop, drop) and `reflog`.
- `rm`, `show`, `tag` (lightweight and annotated), `branch`, `switch` and
  `checkout`, with git's checks for local changes.
- `diff` with Myers line diff, hunks, `--staged`, revisions and paths.

### Fixed

- `log` lists tags and branches of one commit in the order git uses:
  HEAD first, then tags and branches in reverse name order.

## [0.1.0] - 2026-07-15

First tagged version. The old toy that copied files is gone.

### Added

- Blob, tree, commit and tag objects with git's encoding and SHA-1 ids,
  stored as loose zlib files.
- Refs, HEAD (also detached) and reflogs. The index uses git's binary
  format (version 2).
- Revision syntax: `HEAD`, `~N`, `^N`, `^{tree}`, `@{N}` and short ids.
- Commands: `init`, `add`, `commit`, `status`, `log`, `cat-file`,
  `hash-object`, `ls-files` and `config`.
- GitHub Actions for build, test, lint and a coverage gate.

### Fixed

- `add` handles a directory that was replaced by a file, which used to
  fail with `not a directory` and leave a stale index entry.

[1.0.0]: https://github.com/sinachaichi/git-simulator/releases/tag/v1.0.0
[0.5.0]: https://github.com/sinachaichi/git-simulator/releases/tag/v0.5.0
[0.1.0]: https://github.com/sinachaichi/git-simulator/releases/tag/v0.1.0

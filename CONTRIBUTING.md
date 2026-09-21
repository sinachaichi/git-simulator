# Contributing

Thanks for looking. This is a learning project, so clear code and honest
tests matter more than speed.

## Setup

You need Go 1.22 or newer. Real `git` is optional but recommended, since
some tests compare against it. Optional tools:

```
go install mvdan.cc/gofumpt@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
pip install pre-commit && pre-commit install
```

```
make test    # all tests
make lint    # golangci-lint
make fmt     # gofumpt
make cover   # coverage summary, CI wants at least 80 percent
```

Run `make test lint` before you push. Every commit should leave the tests
green.

## Making a change

1. Branch from `main`. Names look like `feat/three-way-merge`,
   `fix/index-stale-entries`, `docs/readme`, `test/e2e-binary`,
   `refactor/...`, `chore/...` or `ci/...`.
2. Keep commits small. One idea per commit.
3. Merge back with `git merge --no-ff` so the branch shows up in the log.

## Commit messages

```
<type>(<scope>): <what it does, lowercase, imperative, no period>
```

Types are `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`,
`build` and `perf`. The scope is the area you touched, such as `merge`,
`index`, `cli` or `xray`.

For `feat`, `fix`, `refactor` and `perf`, add a body with two parts:

```
**Before**

What the code did, or could not do. Name the functions and files.

**After**

- What it does now, with the functions and files in backticks.
- Edge cases and why you picked this design.
```

Small changes (a typo, a rename, a tiny fix) only need the subject line.

## Adding a command

1. Write the logic in `internal/porcelain` as a function that takes a
   `*repo.Repo` and returns values. Do not print there.
2. Add a `cmdXxx` function in `internal/cli` and put it in the table in
   `commandTable`. It parses flags with `newFlags` and `parseFlags`.
3. If it can fail in a way a beginner will hit, add a case to
   `internal/friendly` with a `Why` and a `Try`.
4. Test the porcelain function directly, then test the command with a
   session in `internal/cli/cli_test.go`.
5. If real git has the same command, add a differential test: build the
   same repo, run git on a copy, and compare ids or text.
6. Add a row to the table in the readme.

## Adding a lesson

Lessons live in `internal/lessons/catalog.go`. A lesson has a `Setup` and
steps with a task, a hint and a `Check`. A check must look at the
repository, not at what the user typed. Add a test in `lessons_test.go`
that solves the lesson with porcelain calls, one step at a time.

## Style

- Plain English in comments, docs and messages. Short sentences. No
  marketing words.
- Comments say why, not what.
- Errors are values: return them, wrap them with `%w`, and give the ones
  users can hit a clear message.
- No dead code and no TODO stubs.

# Architecture

This is a map of the code for someone who wants to read it or change it.
The layers are small on purpose and each one only knows about the layers
below it.

## Layers

```
cmd/hood            main: picks the shell or a one-shot command
internal/repl       interactive shell (prompt, history, completion, file helpers)
internal/cli        argument parsing, command table, all printing
internal/lessons    lesson definitions, checker, saved progress
internal/friendly   error to "why" and "try" text
internal/xray       renders recorded events
internal/graph      commit graph lanes
internal/porcelain  the commands: add, commit, merge, stash, ...
internal/merge      three-way text merge and tree merge
internal/diff       Myers diff, hunks, patch rendering
internal/history    commit graph walks, merge bases
internal/revparse   "HEAD~2", "main^2", "stash@{1}", short ids
internal/repo       one repository: paths, config, identity, observer
internal/index      staging area in git's binary index format
internal/refs       branches, tags, HEAD, reflogs
internal/odb        loose object store
internal/tree       tree objects <-> flat file lists
internal/objects    blob, tree, commit, tag, ids and hashing
internal/workdir    the files on disk
internal/events     what the layers report
internal/term       colors
internal/testutil   helpers for tests, including running real git
```

`porcelain` functions take a `*repo.Repo` and return plain values. They
never print. `cli` turns those values into text. That split is why the
same functions serve the shell, the one-shot commands and the lessons.

## On disk

A simulated repository is a folder with a `.hood` directory inside. The
contents are the same as `.git`:

```
.hood/
  HEAD                 "ref: refs/heads/main" or a commit id
  config               core settings, user.name, user.email
  index                binary index, version 2
  objects/ab/cdef...   zlib compressed, "<type> <size>\0<body>"
  refs/heads/<name>    one file per branch
  refs/tags/<name>     one file per tag
  refs/stash           the newest stash commit
  logs/HEAD            reflog, one line per move
  logs/refs/...        reflog per ref, the stash stack lives in logs/refs/stash
  ORIG_HEAD, MERGE_HEAD, MERGE_MSG, CHERRY_PICK_HEAD, REVERT_HEAD
```

Because the format is git's, `git --git-dir=.hood ...` can read it. The
differential tests rely on that.

`repo.Init` refuses to run when the folder or one of its parents has a
`.git`, so the simulator can not end up inside a real project.

## Objects

`objects` encodes the four object types and hashes them. Trees must be
sorted the way git sorts them, where a directory name compares as if it
ended in `/`. `odb.Store` writes loose objects through a temp file and a
rename and re-hashes on read, so a damaged file is noticed.

The empty tree `4b825dc` is not stored, as in git. Code that needs it has
to treat "no tree" as a special case (see `Show` for root commits).

## The index

`index.Index` keeps entries sorted by path and stage. Stage 0 is normal.
Stages 1, 2 and 3 are the base, ours and theirs sides of a conflict. `Add`
replaces a directory with a file of the same name and the other way round.
`WriteTree` refuses to run while stages 1 to 3 exist, which is how
`commit` refuses to commit unresolved conflicts.

## A command from start to end

`hood commit -m "x"`:

1. `cli.Run` parses global flags, finds `commit` in the command table.
2. `cmdCommit` opens the repo with `Discover`, parses flags and calls
   `porcelain.Commit`.
3. `Commit` loads the index, writes it as trees (`index.WriteTree` and
   `tree.Build`), reads HEAD, builds the commit, writes it and moves the
   branch with `moveHead`, which also writes the reflogs.
4. `cmdCommit` prints `[main abc1234] x`.
5. If X-ray is on, `cli.Run` renders the events the layers reported.

## X-ray

`odb.Store`, `refs.Store` and `workdir.WorkDir` have an `Observer` field.
When it is set they send an `events.Event` for each change they make.
`Repo.SaveIndex` compares the old and new index and reports each path.
`repo.SetObserver` sets all of them at once. With no observer there is one
nil check per call and nothing else.

`xray.Render` groups the events (objects, index, refs, working tree,
state) and prints them. The renderer knows nothing about commands.

## Merging

`history.MergeBases` finds the best common ancestors. `merge.Trees` looks
at every path: if only one side changed it, that side wins without reading
the file. Otherwise `merge.Text` runs a three-way merge on the lines
(diff the base against each side, group changes that touch each other, then
decide per group). Conflicts become markers in the file and stages 1 to 3
in the index. `porcelain.applyMerge` writes the result and is shared by
merge, cherry-pick, revert and stash apply, which only differ in which
trees they pass as base and "theirs".

A merge that stops leaves `MERGE_HEAD` and `MERGE_MSG`. `Commit` reads them,
adds the second parent and removes them. `merge --abort` is a hard reset.

## Graph

`graph.TopoOrder` orders commits like `git log --graph`: a stack where the
last parent of a merge comes out first. `graph.Layout` keeps one lane per
commit that is expected next and returns, for each commit, the cells in
front of it and the connector lines below it. `cli` adds the labels.

## Lessons

A lesson has a `Setup` that builds a repository with the porcelain
functions and a list of steps. A step has a `Check` that looks at the
repository. Progress (how many steps are done) is stored in
`progress.json`. Checks run from the saved step on, never from the start,
because a later step may undo what an earlier one needed.

## Tests

- Unit tests sit next to the code.
- Differential tests (`testutil.RequireGit`) run real git on the same
  input, or on a copy of a simulated repository, and compare ids, trees,
  output text or exit codes. They skip when git is missing.
- `internal/cli` tests run whole sessions against buffers.
- `cmd/hood/e2e_test.go` builds the binary and runs it as a process.

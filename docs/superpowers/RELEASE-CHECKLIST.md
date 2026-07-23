# fluo v0.1.0 Release Checklist (operator-run)

This repo is publish-**prepped**, not published: Phase 8 Task 8 verified
everything below is clean, but no subagent creates the GitHub repo, adds a
remote, pushes, or tags. Those are the operator's steps, run manually. This
checklist enumerates them exactly.

## Pre-flight (already verified by Phase 8 Task 8 — re-run if anything changed since)

From the repo root, in WSL (Go only lives there):

```sh
wsl -e bash -lc 'cd /mnt/c/Users/dread/source/fluo && \
  go build ./... && \
  go vet ./... && \
  gofmt -l . && \
  go test -count=1 ./...'
```

Expect: `go build`/`go vet` silent (no output = clean), `gofmt -l .` prints
nothing (no unformatted files), `go test` prints `ok` for every package
(including `render/gl` and `render/gl/gltest`, the golden-image suite).

Also confirm (already true as of this checklist):
- `go.mod`: `module github.com/0xdreadnaught/fluo`, `go 1.23`.
- `LICENSE`: MIT, copyright 0xdreadnaught.
- Every package (17: root + `anim`/`app`/`bind`/`controls`/`core`/`input`/
  `render`/`render/gl`/`render/gl/gltest`/`text`/`theme`/`timers`/
  `cmd/fluo-demo`/`cmd/fluo-gallery`/`examples/counter`/`examples/form`/
  `examples/todo`) has real package-doc content (`go doc <pkg>` shows more
  than a bare header).

## 1. Create the GitHub repository

Create an empty repo at `github.com/0xdreadnaught/fluo` (matching the
`go.mod` module path exactly — required for `go get`/`go install` to resolve
it). Via the GitHub web UI, or:

```sh
gh repo create 0xdreadnaught/fluo --public --description "Fluent/WinUI-styled retained-mode GUI toolkit for OpenGL apps in Go" --source=. --remote=origin
```

If created via the web UI instead (no `--source`/`--remote`), wire the
remote manually (step 2).

## 2. Wire the remote (skip if `gh repo create --source --remote` already did this)

```sh
git remote add origin https://github.com/0xdreadnaught/fluo.git
git remote -v   # confirm origin points at the right URL
```

## 3. Push the branch

This work happened on `phase-8`. Decide whether `main` should be created
from `phase-8` (fast-forward/merge) before pushing, per the operator's own
branch-hygiene preference — this checklist does not presume which. Once the
local branch that should become the published `main` is ready:

```sh
git push -u origin main
```

## 4. Tag v0.1.0

```sh
git tag -a v0.1.0 -m "fluo v0.1.0"
git push origin v0.1.0
```

(`git push --tags` also works if there are no other unpushed tags to worry
about excluding.)

## 5. Verify the published module resolves

Once pushed and tagged, confirm the module is fetchable by its import path
(run from OUTSIDE this repo, e.g. a scratch dir, so it exercises the real
network path rather than a local replace/cache hit):

```sh
go install github.com/0xdreadnaught/fluo/cmd/fluo-gallery@v0.1.0
```

or, without installing a binary:

```sh
go list -m github.com/0xdreadnaught/fluo@v0.1.0
```

Both require the Go module proxy (proxy.golang.org, or GONOSUMCHECK/GOPROXY
as configured) to have indexed the new tag — allow a short delay after
pushing the tag if the first attempt 404s.

## Done

At that point fluo v0.1.0 is public and `go get github.com/0xdreadnaught/fluo`
works for any consumer. Nothing past this point (issue triage, v0.2 planning,
etc.) is in scope for this checklist.

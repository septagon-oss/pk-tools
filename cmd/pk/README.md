# pk — PlatformKit OSS CLI

`pk` is the developer CLI for the PlatformKit OSS workspace. It is intentionally
small: three subcommands that cover the day-1 needs of someone exploring or
hacking on the v0.1.0 essentials pack.

## Install

```bash
go install github.com/septagon-oss/pk-tools/cmd/pk@latest
```

For local development inside the OSS workspace, build directly:

```bash
cd pk-tools
go build ./cmd/pk
./pk --help
```

## Commands

### `pk doctor`

Runs a quick environment health check:

- Go toolchain version is `>= 1.22`
- `modernc.org/sqlite` is declared as a dependency in the current module
- TCP port `:8080` is free (the starter-saas demo binds it)
- `GOPATH` exists and is writable

Each check prints a single `[OK]` or `[FAIL]` line; the command exits non-zero
if any check fails.

### `pk verify`

Runs `go vet ./...` then `go test ./...` in the current working directory and
streams both steps' output. The command exits zero only when both steps pass.

### `pk explain modules`

Prints the OSS module catalog. Each row sources its metadata from the module's
public constants (`ModuleID`, `ModuleName`, `ModuleDescription`, `ModuleVersion`),
so the listing stays in lockstep with the modules themselves.

Add `--json` to get a machine-readable payload suitable for piping into other
tools:

```bash
pk explain modules --json | jq '.[].id'
```

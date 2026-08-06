# Agent orientation

> Part of [PlatformKit](https://github.com/septagon-oss/platformkit) — the open-source Go backend for multi-tenant SaaS.

## Where things live

- `pkg/` — cliapp, scaffold, tui

## Working rules

- `go build ./... && go test ./...` (and `make check`/`make ci` where
  defined) must stay green before opening a PR.
- Public contracts live in `pkg/` exported APIs; internal helpers stay
  unexported. Docs and the README are the advertised surface — keep them
  truthful when behavior changes.

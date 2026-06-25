# pk-tools Charter

## Purpose

CLI, TUI, and developer workflow primitives for PlatformKit OSS. Reusable command-shell behaviour: root command construction, JSON output helpers, context-aware command visibility, and terminal rendering.

## In Scope

- CLI application framework (`pkg/cliapp`): root command, subcommand registration, output formatting
- Terminal UI (`pkg/tui`): status bars, tables, spinner, progress indicators
- Scaffold generators (`pkg/scaffold`): module, entity, and feature scaffolding (experimental)
- The `pk` command (`cmd/pk`): doctor, verify, explain workflows

## Out of Scope

- Product-specific commands (stay in downstream applications)
- Build, deploy, or release automation
- IDE plugins or editor extensions
- CI/CD workflow definitions

## Dependencies

- `github.com/septagon-oss/pk-core` — module contracts
- `github.com/septagon-oss/pk-modules` — module introspection
- `github.com/spf13/cobra` — CLI framework

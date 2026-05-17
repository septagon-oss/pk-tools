# pk-tools

Shared CLI and terminal UI primitives for PlatformKit.

This repository intentionally contains reusable command-shell behavior only:
root command construction, JSON output helpers, context-aware command visibility
hooks, and terminal-aware status/table rendering. Product-specific commands stay
in downstream applications and compose these packages.

## Verify

```bash
make verify
make staticcheck
```

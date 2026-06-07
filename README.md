# pk-tools

[![Go Reference](https://pkg.go.dev/badge/github.com/septagon-oss/pk-tools.svg)](https://pkg.go.dev/github.com/septagon-oss/pk-tools)
[![CI](https://github.com/septagon-oss/pk-tools/actions/workflows/go.yml/badge.svg)](https://github.com/septagon-oss/pk-tools/actions/workflows/go.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

pk-tools provides the shared CLI and terminal-UI primitives for the OSS PlatformKit family: root-command assembly, JSON output helpers, context-aware command visibility, and terminal-aware status and table rendering. It deliberately ships reusable command-shell behavior only — product-specific commands live in downstream applications and compose these packages.

## Install

```bash
go get github.com/septagon-oss/pk-tools@v0.1.0
```

## Usage

```go
package main

import (
	"github.com/septagon-oss/pk-tools/pkg/cliapp"
	"github.com/spf13/cobra"
)

func main() {
	root := cliapp.NewRoot(cliapp.RootOptions{
		Use:     "demo",
		Short:   "Demo CLI built on pk-tools primitives",
		Version: "v0.1.0",
		Commands: []*cobra.Command{
			{Use: "greet", RunE: func(cmd *cobra.Command, _ []string) error {
				cmd.Println("hello from pk-tools")
				return nil
			}},
		},
	})

	if err := cliapp.Execute(root); err != nil {
		panic(err)
	}
}
```

## Current Surface

- `pkg/cliapp` — root `*cobra.Command` assembly via `NewRoot`/`RootOptions`, `Execute`, argument helpers (`HasArg`, `ShowAllCommands`, `AddHiddenBoolFlag`), and JSON output helpers (`WriteJSON`, `SortedMapKeys`).
- `pkg/tui` — terminal-aware `Renderer` with `Info`/`Success`/`Warn`/`Error` status lines, `Table` rendering, `CommandStart` banners, and a configurable `Palette` with `NO_COLOR`-aware color detection.
- `pkg/scaffold` — module, entity, feature, and project generators used to compose PlatformKit OSS applications.
- `cmd/pk` — the `pk` developer CLI (`doctor`, `verify`, `explain modules`) built on these primitives.

## Verify

```bash
make verify   # go test + go vet + staticcheck + race
```

## License

Apache-2.0. See [LICENSE](LICENSE).

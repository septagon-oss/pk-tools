module github.com/septagon-oss/pk-tools

go 1.26

require (
	github.com/septagon-oss/pk-design v0.3.0
	github.com/septagon-oss/pk-modules v0.18.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/mod v0.40.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/septagon-oss/pk-core v0.1.0 // indirect
	github.com/septagon-oss/pk-shared v0.2.0 // indirect
	github.com/septagon-oss/pk-ui v0.3.0 // indirect
	github.com/septagon-oss/styleengine v0.1.0 // indirect
	github.com/septagon-oss/tw v0.2.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tdewolff/minify/v2 v2.24.14 // indirect
	github.com/tdewolff/parse/v2 v2.8.14 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	maragu.dev/gomponents v1.3.0 // indirect
	modernc.org/libc v1.74.3 // indirect
	modernc.org/sqlite v1.54.0 // indirect
)

retract (
	[v0.3.0, v0.3.1] // broken: pinned pk-modules v0.15.0, which lacks pkg/branding that cmd/pk imports
	v0.0.0 // broken: contained local replace directives
)

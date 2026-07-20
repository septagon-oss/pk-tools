// Package tui contains terminal-aware rendering primitives for PlatformKit CLIs.
package tui

// Implements: REQ-015.
// Per: ADR-0021.
// Discipline: C-14.
// renderer.go owns low-level terminal rendering primitives without binding the
// CLI layer to a specific command framework.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Palette holds the ANSI escape sequences a Renderer uses to colorize output.
// Each field is the prefix written before a styled span; Reset is appended
// after it. A zero Palette produces uncolored output.
type Palette struct {
	Accent  string
	Success string
	Warning string
	Error   string
	Muted   string
	Bold    string
	Reset   string
}

// DefaultPalette is the standard ANSI color scheme applied by New.
var DefaultPalette = Palette{
	Accent:  "\x1b[36m",
	Success: "\x1b[32m",
	Warning: "\x1b[33m",
	Error:   "\x1b[31m",
	Muted:   "\x1b[2m",
	Bold:    "\x1b[1m",
	Reset:   "\x1b[0m",
}

// Renderer writes terminal-aware status lines and tables. Info and Success
// messages go to Out; Warn and Error messages go to Err. When Color is true,
// styled output is wrapped in the escape sequences from Palette.
type Renderer struct {
	Out     io.Writer
	Err     io.Writer
	Palette Palette
	Color   bool
}

// New returns a Renderer that writes to out and err using DefaultPalette. Nil
// writers are replaced with io.Discard, and Color is enabled automatically
// unless the environment opts out (NO_COLOR, CLICOLOR=0, or TERM=dumb).
func New(out, err io.Writer) Renderer {
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	return Renderer{
		Out:     out,
		Err:     err,
		Palette: DefaultPalette,
		Color:   colorEnabled(),
	}
}

// Info writes a printf-style "info" status line to the renderer's Out stream.
func (r Renderer) Info(format string, args ...any) {
	r.line(r.Out, r.style("info", r.Palette.Accent), format, args...)
}

// Success writes a printf-style "ok" status line to the renderer's Out stream.
func (r Renderer) Success(format string, args ...any) {
	r.line(r.Out, r.style("ok", r.Palette.Success), format, args...)
}

// Warn writes a printf-style "warn" status line to the renderer's Err stream.
func (r Renderer) Warn(format string, args ...any) {
	r.line(r.Err, r.style("warn", r.Palette.Warning), format, args...)
}

// Error writes a printf-style "error" status line to the renderer's Err stream.
func (r Renderer) Error(format string, args ...any) {
	r.line(r.Err, r.style("error", r.Palette.Error), format, args...)
}

// CommandStart writes a one-line banner to Out announcing that an external
// command name is about to run with args in dir. An empty prefix defaults to
// "Running" and is accent-colored when Color is enabled.
func (r Renderer) CommandStart(prefix, name string, args []string, dir string) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "Running"
	}
	if r.Color {
		prefix = r.style(prefix, r.Palette.Accent)
	}
	_, _ = fmt.Fprintf(r.Out, "%s %s %v in %s\n", prefix, name, args, dir)
}

// Table writes headers and rows to Out as a tab-aligned table. Each inner slice
// of rows is one table row; cells are separated by tabs and column-aligned via
// a tabwriter. An empty headers slice omits the header line.
func (r Renderer) Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	if len(headers) > 0 {
		_, _ = fmt.Fprintln(w, strings.Join(headers, "\t"))
	}
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}

func (r Renderer) line(w io.Writer, label string, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "%s %s\n", label, fmt.Sprintf(format, args...))
}

func (r Renderer) style(value, color string) string {
	if !r.Color {
		return value
	}
	return color + value + r.Palette.Reset
}

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

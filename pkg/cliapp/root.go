// Package cliapp provides composable Cobra application primitives.
package cliapp

// root.go owns root-command assembly and shared argument helpers for OSS
// PlatformKit CLIs.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// RootOptions configures the construction of a root Cobra command via NewRoot.
// All string fields are trimmed before use. Use and every command are required;
// invalid static composition panics. ShowAllEnv names an environment variable consulted by
// ShowAllCommands to decide whether hidden commands should be revealed.
type RootOptions struct {
	Use        string
	Short      string
	Long       string
	Version    string
	Commands   []*cobra.Command
	ShowAllEnv string
}

// NewRoot builds a root *cobra.Command from the supplied RootOptions. It trims
// the Use, Short, Long, and Version fields and attaches every command in
// Commands as a subcommand. Invalid static composition panics. The returned
// command is ready to pass to Execute.
func NewRoot(options RootOptions) *cobra.Command {
	use := strings.TrimSpace(options.Use)
	if use == "" {
		panic("cliapp.NewRoot: Use is required")
	}
	root := &cobra.Command{
		Use:     use,
		Short:   strings.TrimSpace(options.Short),
		Long:    strings.TrimSpace(options.Long),
		Version: strings.TrimSpace(options.Version),
	}
	for index, child := range options.Commands {
		if child == nil {
			panic("cliapp.NewRoot: command at index " + strconv.Itoa(index) + " is nil")
		}
		root.AddCommand(child)
	}
	return root
}

// Execute runs the supplied root command, returning any error its execution
// produces. A nil root is an invalid execution request.
func Execute(root *cobra.Command) error {
	if root == nil {
		return errors.New("root command is required")
	}
	return root.Execute()
}

// ShowAllCommands reports whether hidden commands should be displayed. It
// returns true when the environment variable named by envName is set to "1",
// or when args contains the "--all-commands" flag.
func ShowAllCommands(envName string, args []string) bool {
	if strings.TrimSpace(envName) != "" && os.Getenv(envName) == "1" {
		return true
	}
	return HasArg(args, "--all-commands")
}

// HasArg reports whether args contains target, either as an exact match or as
// a "target=value" flag assignment.
func HasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target || strings.HasPrefix(arg, target+"=") {
			return true
		}
	}
	return false
}

// AddHiddenBoolFlag registers a persistent bool flag on cmd bound to target and
// immediately marks it hidden, so it works but does not appear in help output.
// Invalid static flag composition panics.
func AddHiddenBoolFlag(cmd *cobra.Command, target *bool, name string, value bool, usage string) {
	if cmd == nil {
		panic("cliapp.AddHiddenBoolFlag: command is required")
	}
	if target == nil {
		panic("cliapp.AddHiddenBoolFlag: target is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		panic("cliapp.AddHiddenBoolFlag: name is required")
	}
	cmd.PersistentFlags().BoolVar(target, name, value, usage)
	if err := cmd.PersistentFlags().MarkHidden(name); err != nil {
		panic(err)
	}
}

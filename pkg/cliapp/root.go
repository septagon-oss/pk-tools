// Package cliapp provides composable Cobra application primitives.
package cliapp

// root.go owns root-command assembly and shared argument helpers for OSS
// PlatformKit CLIs.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type RootOptions struct {
	Use        string
	Short      string
	Long       string
	Version    string
	Commands   []*cobra.Command
	ShowAllEnv string
}

func NewRoot(options RootOptions) *cobra.Command {
	root := &cobra.Command{
		Use:     strings.TrimSpace(options.Use),
		Short:   strings.TrimSpace(options.Short),
		Long:    strings.TrimSpace(options.Long),
		Version: strings.TrimSpace(options.Version),
	}
	for _, child := range options.Commands {
		if child != nil {
			root.AddCommand(child)
		}
	}
	return root
}

func Execute(root *cobra.Command) error {
	if root == nil {
		return nil
	}
	return root.Execute()
}

func ShowAllCommands(envName string, args []string) bool {
	if strings.TrimSpace(envName) != "" && os.Getenv(envName) == "1" {
		return true
	}
	return HasArg(args, "--all-commands")
}

func HasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target || strings.HasPrefix(arg, target+"=") {
			return true
		}
	}
	return false
}

func AddHiddenBoolFlag(cmd *cobra.Command, target *bool, name string, value bool, usage string) {
	if cmd == nil {
		return
	}
	cmd.PersistentFlags().BoolVar(target, name, value, usage)
	_ = cmd.PersistentFlags().MarkHidden(name)
}

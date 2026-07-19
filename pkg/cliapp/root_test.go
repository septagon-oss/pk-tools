package cliapp

// root_test.go validates command assembly, output routing, and argument
// detection behavior used by OSS PlatformKit CLIs.
//
// ADR: ADR-0029 (file purpose declaration).
// Convention: C-14 (every Go file declares its purpose).

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestShowAllCommandsHonorsEnvAndArgs(t *testing.T) {
	t.Setenv("PK_TEST_SHOW_ALL", "")
	if ShowAllCommands("PK_TEST_SHOW_ALL", []string{"modules"}) {
		t.Fatal("expected false without env or flag")
	}
	t.Setenv("PK_TEST_SHOW_ALL", "1")
	if !ShowAllCommands("PK_TEST_SHOW_ALL", []string{"modules"}) {
		t.Fatal("expected env override")
	}
	t.Setenv("PK_TEST_SHOW_ALL", "")
	if !ShowAllCommands("PK_TEST_SHOW_ALL", []string{"--all-commands"}) {
		t.Fatal("expected flag override")
	}
}

func TestNewRootExecutesChildCommand(t *testing.T) {
	var out bytes.Buffer
	root := NewRoot(RootOptions{
		Use: "pk",
		Commands: []*cobra.Command{{
			Use: "hello",
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Print("world")
			},
		}},
	})
	root.SetOut(&out)
	root.SetArgs([]string{"hello"})

	if err := Execute(root); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.String() != "world" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRootCompositionRejectsMissingCommands(t *testing.T) {
	if err := Execute(nil); err == nil {
		t.Fatal("Execute(nil) reported success")
	}

	t.Run("root use", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewRoot accepted an empty Use")
			}
		}()
		NewRoot(RootOptions{})
	})

	t.Run("child command", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewRoot skipped a nil child")
			}
		}()
		NewRoot(RootOptions{Use: "pk", Commands: []*cobra.Command{nil}})
	})

	t.Run("hidden flag command", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("AddHiddenBoolFlag accepted a nil command")
			}
		}()
		var target bool
		AddHiddenBoolFlag(nil, &target, "hidden", false, "hidden")
	})
}

func TestWriteJSONUsesCommandWriter(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := WriteJSON(cmd, map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["status"] != "ok" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

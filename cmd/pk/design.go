package main

// Implements: REQ-011, REQ-015.
// Per: ADR-0076.
// Discipline: C-14.
// design.go exposes the OSS native-Figma delivery and guarded DTCG receive
// workflow. Receives are dry-run by default and compare the planned source
// digest immediately before a same-directory atomic replacement.
//
// ADR: ADR-0029 (file purpose declaration), ADR-0076 (layered design delivery).
// Convention: C-10 (shared builders return errors), C-14 (every Go file declares its purpose).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/septagon-oss/pk-design/pkg/delivery"
	"github.com/septagon-oss/pk-design/pkg/handoff"
	"github.com/spf13/cobra"
)

func newDesignCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "design",
		Short: "Deliver and receive the editable PlatformKit OSS design system",
	}
	command.AddCommand(newDesignExportCmd(), newDesignReceiveTokensCmd())
	return command
}

func newDesignExportCmd() *cobra.Command {
	var output string
	var generatedAt string
	command := &cobra.Command{
		Use:   "export",
		Short: "Export native Figma Variables bound to the canonical OSS DTCG source",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(output) == "" {
				return fmt.Errorf("design export: --out is required")
			}
			at, err := parseGeneratedAt(generatedAt, time.Now)
			if err != nil {
				return err
			}
			bundle, err := delivery.DefaultFigmaVariables(at)
			if err != nil {
				return fmt.Errorf("design export: %w", err)
			}
			raw, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				return fmt.Errorf("design export: marshal bundle: %w", err)
			}
			raw = append(raw, '\n')
			if err := atomicWriteFile(output, raw, nil); err != nil {
				return fmt.Errorf("design export: %w", err)
			}
			digest, err := bundle.Snapshot.Digest()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"design export: wrote native editable profile %s (%s) to %s\n",
				bundle.Snapshot.Profile.ID,
				digest,
				output,
			)
			return err
		},
	}
	command.Flags().StringVar(&output, "out", "", "output path for pk.design.figma-variables.v2 JSON")
	command.Flags().StringVar(
		&generatedAt,
		"generated-at",
		"",
		"RFC3339 evidence timestamp (defaults to current UTC time)",
	)
	return command
}

func newDesignReceiveTokensCmd() *cobra.Command {
	var input string
	var source string
	var format string
	var write bool
	command := &cobra.Command{
		Use:   "receive-tokens",
		Short: "Plan or commit native Figma token changes to canonical OSS DTCG",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDesignReceiveTokens(
				cmd.OutOrStdout(),
				input,
				source,
				format,
				write,
			)
		},
	}
	command.Flags().StringVar(&input, "in", "", "pk.design.token-changes.v1 JSON exported by the Figma plugin")
	command.Flags().StringVar(&source, "source", "", "path to pk-design/pkg/themes/default.tokens.json")
	command.Flags().StringVar(&format, "format", "human", "plan output: human | json")
	command.Flags().BoolVar(&write, "write", false, "atomically commit the validated DTCG replacement")
	return command
}

func runDesignReceiveTokens(
	out io.Writer,
	input string,
	source string,
	format string,
	write bool,
) error {
	input = strings.TrimSpace(input)
	source = strings.TrimSpace(source)
	if input == "" {
		return fmt.Errorf("design receive-tokens: --in is required")
	}
	if source == "" {
		return fmt.Errorf("design receive-tokens: --source is required")
	}
	changeBytes, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("design receive-tokens: read changes: %w", err)
	}
	var changes handoff.ChangeSet
	if err := json.Unmarshal(changeBytes, &changes); err != nil {
		return fmt.Errorf("design receive-tokens: parse changes: %w", err)
	}
	sourceBytes, err := readRegularFile(source)
	if err != nil {
		return fmt.Errorf("design receive-tokens: %w", err)
	}
	plan, err := delivery.PlanDefaultChanges(sourceBytes, changes)
	if err != nil {
		return fmt.Errorf("design receive-tokens: %w", err)
	}
	switch strings.TrimSpace(format) {
	case "human":
		if _, err := fmt.Fprintf(
			out,
			"design receive-tokens: %d change(s), %s -> %s (%s)\n",
			plan.Changes,
			plan.CurrentDigest,
			mustSnapshotDigest(plan.NextSnapshot),
			map[bool]string{false: "dry-run", true: "write"}[write],
		); err != nil {
			return err
		}
	case "json":
		raw, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("design receive-tokens: marshal plan: %w", err)
		}
		if _, err := fmt.Fprintf(out, "%s\n", raw); err != nil {
			return err
		}
	default:
		return fmt.Errorf("design receive-tokens: unsupported --format %q", format)
	}
	if !write {
		return nil
	}
	if err := atomicWriteFile(source, plan.Contents, &plan.SourceDigest); err != nil {
		return fmt.Errorf("design receive-tokens: commit: %w", err)
	}
	_, err = fmt.Fprintf(
		out,
		"design receive-tokens: committed %s as one validated atomic replacement\n",
		source,
	)
	return err
}

func parseGeneratedAt(value string, now func() time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("design export: --generated-at must be RFC3339: %w", err)
	}
	return parsed.UTC(), nil
}

func mustSnapshotDigest(snapshot handoff.Snapshot) string {
	digest, err := snapshot.Digest()
	if err != nil {
		return "invalid:" + err.Error()
	}
	return digest
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return raw, nil
}

// atomicWriteFile stages beside target so rename remains atomic. When
// expectedDigest is provided, an existing regular target must still have that
// exact digest immediately before replacement.
func atomicWriteFile(path string, contents []byte, expectedDigest *string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent := filepath.Dir(absolute)
	info, statErr := os.Lstat(absolute)
	mode := os.FileMode(0o644)
	if statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", absolute)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	if expectedDigest != nil {
		if statErr != nil {
			return fmt.Errorf("%s disappeared after planning", absolute)
		}
		current, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		if got := digestBytes(current); got != *expectedDigest {
			return fmt.Errorf(
				"%s changed after planning (%s != %s); export and retry",
				absolute,
				got,
				*expectedDigest,
			)
		}
	}
	staged, err := os.CreateTemp(parent, ".pk-design-next-*")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(stagedPath)
		}
	}()
	if _, err := staged.Write(contents); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Chmod(mode); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Rename(stagedPath, absolute); err != nil {
		return err
	}
	committed = true
	directory, err := os.Open(parent)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

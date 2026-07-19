// Validates: REQ-016.
// Per: ADR-0061.
// Discipline: C-14.
//go:build integration

package scaffold

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildProjectCompilesAgainstCurrentWorkspace(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test source path")
	}
	workspaceRoot, ok := findPlatformKitWorkspace(filepath.Dir(sourceFile))
	if !ok {
		t.Skip("PlatformKit workspace repositories are not available")
	}

	dependencies, profile, err := platformKitWorkspaceDependencies(workspaceRoot)
	if err != nil {
		t.Fatalf("resolve workspace dependency graph: %v", err)
	}
	result, err := BuildProject(ComposeConfig{
		Name:          "Scaffold Compile",
		ModulePath:    "example.com/scaffold-compile",
		Description:   "Compile contract for the canonical project scaffold",
		Modules:       []string{"health_management"},
		Dependencies:  dependencies,
		ImportProfile: profile,
	})
	if err != nil {
		t.Fatalf("BuildProject failed: %v", err)
	}

	projectRoot := t.TempDir()
	if err := WriteFiles(WriteOptions{BaseDir: projectRoot, Files: result.Files}); err != nil {
		t.Fatalf("write generated project: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	env := append(os.Environ(),
		"GOWORK=off",
		"GOTOOLCHAIN=local",
		"GOTMPDIR="+filepath.Join(workspaceRoot, ".tmp-go-tmp"),
	)
	for _, args := range [][]string{{"mod", "tidy"}, {"mod", "vendor"}, {"test", "-mod=vendor", "."}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = projectRoot
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("generated project compile timed out: %v\n%s", ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("generated project command go %v failed: %v\n%s", args, err, output)
		}
	}
}

func platformKitWorkspaceDependencies(root string) (ProjectDependencies, ImportProfile, error) {
	workBody, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		return ProjectDependencies{}, ImportProfile{}, fmt.Errorf("read go.work: %w", err)
	}

	dependencies := ProjectDependencies{}
	profile := ImportProfile{}
	backendRoot := filepath.Join(root, "core", "platformkit-backend-kit")
	businessRoot := filepath.Join(root, "modules", "platformkit-business-modules")
	inUseBlock := false
	for _, rawLine := range strings.Split(string(workBody), "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "use (":
			inUseBlock = true
			continue
		case inUseBlock && line == ")":
			inUseBlock = false
			continue
		case !inUseBlock || !strings.HasPrefix(line, "./"):
			continue
		}

		localPath := filepath.Join(root, strings.TrimPrefix(line, "./"))
		modBody, err := os.ReadFile(filepath.Join(localPath, "go.mod"))
		if err != nil {
			return ProjectDependencies{}, ImportProfile{}, fmt.Errorf("read %s/go.mod: %w", line, err)
		}
		modulePath := ""
		for _, rawModLine := range strings.Split(string(modBody), "\n") {
			modLine := strings.TrimSpace(rawModLine)
			if strings.HasPrefix(modLine, "module ") {
				modulePath = strings.TrimSpace(strings.TrimPrefix(modLine, "module "))
				break
			}
		}
		if modulePath == "" {
			return ProjectDependencies{}, ImportProfile{}, fmt.Errorf("%s/go.mod has no module directive", line)
		}
		switch localPath {
		case backendRoot:
			profile.BackendKit = modulePath
			dependencies.BackendKit = GoModuleSource{Version: "v0.0.0", ReplacePath: localPath}
		case businessRoot:
			profile.BusinessModules = modulePath
			dependencies.BusinessModules = GoModuleSource{Version: "v0.0.0", ReplacePath: localPath}
		default:
			dependencies.AdditionalReplacements = append(dependencies.AdditionalReplacements, GoModuleReplacement{
				ModulePath: modulePath,
				LocalPath:  localPath,
			})
		}
	}
	if dependencies.BackendKit.ReplacePath == "" || dependencies.BusinessModules.ReplacePath == "" {
		return ProjectDependencies{}, ImportProfile{}, fmt.Errorf("go.work must include backend kit and business modules")
	}
	sort.Slice(dependencies.AdditionalReplacements, func(i, j int) bool {
		return dependencies.AdditionalReplacements[i].ModulePath < dependencies.AdditionalReplacements[j].ModulePath
	})
	return dependencies, profile, nil
}

func findPlatformKitWorkspace(start string) (string, bool) {
	for current := start; ; current = filepath.Dir(current) {
		backendMod := filepath.Join(current, "core", "platformkit-backend-kit", "go.mod")
		modulesMod := filepath.Join(current, "modules", "platformkit-business-modules", "go.mod")
		if fileExists(backendMod) && fileExists(modulesMod) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

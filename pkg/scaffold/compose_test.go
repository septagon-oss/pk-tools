package scaffold

import (
	"strings"
	"testing"
)

func TestBuildProject(t *testing.T) {
	result, err := BuildProject(ComposeConfig{
		Name:       "testapp",
		ModulePath: "github.com/myorg/testapp",
		Modules:    []string{"auth_management", "user_management"},
		Port:       "8080",
	})
	if err != nil {
		t.Fatalf("BuildProject failed: %v", err)
	}

	if result.Name != "testapp" {
		t.Errorf("expected name testapp, got %s", result.Name)
	}
	if result.DBName != "testapp" {
		t.Errorf("expected dbName testapp, got %s", result.DBName)
	}
	if len(result.Files) != 8 {
		t.Errorf("expected 8 files, got %d", len(result.Files))
	}

	expectedPaths := map[string]bool{
		".platformkit/project.json": true,
		"main.go":                   true, "go.mod": true, "config.yaml": true,
		"Dockerfile": true, "docker-compose.yml": true,
		"Makefile": true, "migrations/seed.sql": true,
	}
	for _, f := range result.Files {
		if !expectedPaths[f.Path] {
			t.Errorf("unexpected file path: %s", f.Path)
		}
		if f.Content == "" {
			t.Errorf("empty content for %s", f.Path)
		}
	}
}

func TestBuildProjectDefaults(t *testing.T) {
	result, err := BuildProject(ComposeConfig{
		Name:    "My App",
		Modules: []string{"auth_management"},
	})
	if err != nil {
		t.Fatalf("BuildProject failed: %v", err)
	}

	if result.ModulePath != "github.com/myorg/my-app" {
		t.Errorf("expected default module path, got %s", result.ModulePath)
	}
	if result.DBName != "my_app" {
		t.Errorf("expected db name my_app, got %s", result.DBName)
	}
}

func TestGenerateMainGo(t *testing.T) {
	result := GenerateMainGo("testapp", []string{"auth_management", "user_management"}, "8080")

	if !strings.Contains(result, "package main") {
		t.Error("missing package main")
	}
	if !strings.Contains(result, `WithModules(`) {
		t.Error("missing Bundle-backed module selector")
	}
	if !strings.Contains(result, `"auth_management"`) {
		t.Error("missing auth_management module ID")
	}
	if !strings.Contains(result, `"user_management"`) {
		t.Error("missing user_management module ID")
	}
	if !strings.Contains(result, `application.WithName("testapp")`) {
		t.Error("missing application name")
	}
}

func TestGenerateMainGoAppliesImportProfile(t *testing.T) {
	profile := ImportProfile{
		BackendKit:                 "github.com/acme/platformkit-backend-kit",
		BusinessModules:            "github.com/acme/platformkit-business-modules",
		BackendKitReplacePath:      "../platformkit-backend-kit",
		BusinessModulesReplacePath: "../platformkit-business-modules",
	}

	mainGo := GenerateMainGoWithProfile("testapp", []string{"auth_management"}, "8080", profile)
	if !strings.Contains(mainGo, "github.com/acme/platformkit-backend-kit/app/application") {
		t.Fatal("main.go did not use profiled backend import root")
	}
	if !strings.Contains(mainGo, `platformmodules "github.com/acme/platformkit-business-modules"`) {
		t.Fatal("main.go did not use profiled business modules import root")
	}
	if strings.Contains(mainGo, "example.com/platformkit/") {
		t.Fatal("main.go still contains neutral import roots")
	}
}

func TestGenerateConfigYAML(t *testing.T) {
	result := GenerateConfigYAML("testapp", "A test app", []string{"auth_management"})

	if !strings.Contains(result, "# testapp Configuration") {
		t.Error("missing config header")
	}
	if !strings.Contains(result, "dsn:") {
		t.Error("missing database DSN")
	}
	if !strings.Contains(result, "auth_management:") {
		t.Error("missing module config")
	}
}

func TestGenerateGoMod(t *testing.T) {
	result := GenerateGoMod("github.com/myorg/testapp", false)

	if !strings.Contains(result, "module github.com/myorg/testapp") {
		t.Error("missing module declaration")
	}
	if strings.Contains(result, "replace") {
		t.Error("should not have replace directives without localDev")
	}

	resultLocal := GenerateGoMod("github.com/myorg/testapp", true)
	if !strings.Contains(resultLocal, "replace") {
		t.Error("should have replace directives with localDev")
	}
}

func TestGenerateGoModAppliesImportProfile(t *testing.T) {
	profile := ImportProfile{
		BackendKit:                 "github.com/acme/platformkit-backend-kit",
		BusinessModules:            "github.com/acme/platformkit-business-modules",
		BackendKitReplacePath:      "../platformkit-backend-kit",
		BusinessModulesReplacePath: "../platformkit-business-modules",
	}

	result := GenerateGoModWithProfile("github.com/myorg/testapp", true, profile)
	for _, needle := range []string{
		"github.com/acme/platformkit-backend-kit v0.0.0",
		"github.com/acme/platformkit-business-modules v0.0.0",
		"github.com/acme/platformkit-backend-kit => ../platformkit-backend-kit",
		"github.com/acme/platformkit-business-modules => ../platformkit-business-modules",
	} {
		if !strings.Contains(result, needle) {
			t.Fatalf("profiled go.mod missing %q", needle)
		}
	}
}

func TestGenerateDockerfile(t *testing.T) {
	result := GenerateDockerfile("testapp", "8080")

	if !strings.Contains(result, "FROM golang:1.24-alpine") {
		t.Error("missing builder stage")
	}
	if !strings.Contains(result, "EXPOSE 8080") {
		t.Error("missing port expose")
	}
}

func TestGenerateDockerCompose(t *testing.T) {
	result := GenerateDockerCompose("testapp", "8080", "5432")

	if !strings.Contains(result, "postgres:17-alpine") {
		t.Error("missing postgres service")
	}
	if !strings.Contains(result, "redis:7-alpine") {
		t.Error("missing redis service")
	}
	if !strings.Contains(result, "nats:2-alpine") {
		t.Error("missing nats service")
	}
}

func TestGenerateMakefile(t *testing.T) {
	result := GenerateMakefile("testapp")

	if !strings.Contains(result, "APP_NAME := testapp") {
		t.Error("missing app name")
	}
	if !strings.Contains(result, "docker-up:") {
		t.Error("missing docker-up target")
	}
}

func TestGenerateSeedData(t *testing.T) {
	result := GenerateSeedData("admin@test.com", "TestTenant", []string{"billing_management"})

	if !strings.Contains(result, "admin@test.com") {
		t.Error("missing admin email")
	}
	if !strings.Contains(result, "TestTenant") {
		t.Error("missing tenant name")
	}
	if !strings.Contains(result, "pricing_plans") {
		t.Error("missing billing seed data")
	}
}

func TestGenerateSeedDataDefaults(t *testing.T) {
	result := GenerateSeedData("", "", nil)

	if !strings.Contains(result, "admin@example.com") {
		t.Error("missing default admin email")
	}
	if !strings.Contains(result, "Default") {
		t.Error("missing default tenant name")
	}
}

func TestModuleToFuncName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"auth_management", "AuthManagement"},
		{"user_management", "UserManagement"},
		{"billing_management", "BillingManagement"},
	}
	for _, tt := range tests {
		got := ModuleToFuncName(tt.input)
		if got != tt.expected {
			t.Errorf("ModuleToFuncName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeSQLString(t *testing.T) {
	if EscapeSQLString("it's a test") != "it''s a test" {
		t.Error("failed to escape single quote")
	}
}

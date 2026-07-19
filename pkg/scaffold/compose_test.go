// Validates: REQ-016.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestBuildProject(t *testing.T) {
	result, err := BuildProject(ComposeConfig{
		Name:         "testapp",
		ModulePath:   "github.com/myorg/testapp",
		Modules:      []string{"auth_management", "user_management"},
		Port:         "8080",
		Dependencies: testProjectDependencies(),
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
	if len(result.Files) != 11 {
		t.Errorf("expected 11 files, got %d", len(result.Files))
	}

	expectedPaths := map[string]bool{
		".platformkit/project.json": true,
		".env.example":              true,
		".gitignore":                true,
		".dockerignore":             true,
		"main.go":                   true, "go.mod": true, "config.yaml": true,
		"Dockerfile": true, "docker-compose.yml": true,
		"Makefile":        true,
		"locales/en.json": true,
	}
	for _, f := range result.Files {
		if !expectedPaths[f.Path] {
			t.Errorf("unexpected file path: %s", f.Path)
		}
		if f.Content == "" {
			t.Errorf("empty content for %s", f.Path)
		}
	}
	files := make(map[string]string, len(result.Files))
	for _, file := range result.Files {
		files[file.Path] = file.Content
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", files["main.go"], parser.AllErrors); err != nil {
		t.Fatalf("generated main.go does not parse: %v", err)
	}
	for _, retired := range []string{"migrations/seed.sql", "admin@example.com", "Full system administrator"} {
		for path, content := range files {
			if strings.Contains(content, retired) {
				t.Errorf("%s contains retired privileged seed contract %q", path, retired)
			}
		}
	}
	for _, required := range []string{"POSTGRES_PASSWORD=", "PAAS_AUTH_JWT_SECRET_KEY=", "PAAS_NATS_CONTEXT_AUTH_SECRET="} {
		if !strings.Contains(files[".env.example"], required) {
			t.Errorf(".env.example missing %q", required)
		}
	}
	for _, ignorePath := range []string{".gitignore", ".dockerignore"} {
		if !strings.Contains(files[ignorePath], ".env") {
			t.Errorf("%s does not exclude local secrets", ignorePath)
		}
	}
}

func TestBuildProjectUsesConfiguredPortEverywhere(t *testing.T) {
	result, err := BuildProject(ComposeConfig{
		Name:         "testapp",
		ModulePath:   "github.com/myorg/testapp",
		Modules:      []string{"auth_management"},
		Port:         "4317",
		Dependencies: testProjectDependencies(),
	})
	if err != nil {
		t.Fatalf("BuildProject failed: %v", err)
	}

	files := make(map[string]string, len(result.Files))
	for _, file := range result.Files {
		files[file.Path] = file.Content
	}
	for _, path := range []string{"config.yaml", "Dockerfile", "docker-compose.yml"} {
		if !strings.Contains(files[path], "4317") {
			t.Errorf("%s does not contain configured port", path)
		}
	}
	if strings.Contains(files["config.yaml"], `port: "8080"`) {
		t.Error("config.yaml retained the default HTTP port")
	}
	if files["locales/en.json"] != "[]\n" {
		t.Errorf("unexpected locale catalog: %q", files["locales/en.json"])
	}
}

func TestBuildProjectRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ComposeConfig)
	}{
		{name: "application name", mutate: func(cfg *ComposeConfig) { cfg.Name = "app\nservices:" }},
		{name: "trailing separator", mutate: func(cfg *ComposeConfig) { cfg.Name = "testapp-" }},
		{name: "description", mutate: func(cfg *ComposeConfig) { cfg.Description = "safe\nserver: injected" }},
		{name: "module path", mutate: func(cfg *ComposeConfig) { cfg.ModulePath = "github.com/acme/app\nreplace evil => ../evil" }},
		{name: "application port", mutate: func(cfg *ComposeConfig) { cfg.Port = "8080:9090" }},
		{name: "database port", mutate: func(cfg *ComposeConfig) { cfg.DBPort = "0" }},
		{name: "module name", mutate: func(cfg *ComposeConfig) { cfg.Modules = []string{"AuthManagement"} }},
		{name: "duplicate module", mutate: func(cfg *ComposeConfig) { cfg.Modules = []string{"auth_management", "auth_management"} }},
		{name: "empty modules", mutate: func(cfg *ComposeConfig) { cfg.Modules = nil }},
		{name: "application-owned infrastructure", mutate: func(cfg *ComposeConfig) { cfg.Modules = []string{"infrastructure"} }},
		{name: "backend version", mutate: func(cfg *ComposeConfig) { cfg.Dependencies.BackendKit.Version = "" }},
		{name: "version injection", mutate: func(cfg *ComposeConfig) { cfg.Dependencies.BusinessModules.Version = "v1.0.0\nreplace evil" }},
		{name: "replacement traversal", mutate: func(cfg *ComposeConfig) { cfg.Dependencies.BackendKit.ReplacePath = "../backend/../escape" }},
		{name: "duplicate replacement", mutate: func(cfg *ComposeConfig) {
			cfg.Dependencies.AdditionalReplacements = []GoModuleReplacement{{ModulePath: defaultBackendKitImportRoot, LocalPath: "../backend-copy"}}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ComposeConfig{
				Name:         "testapp",
				ModulePath:   "github.com/acme/testapp",
				Modules:      []string{"auth_management"},
				Dependencies: testProjectDependencies(),
			}
			tt.mutate(&cfg)
			if _, err := BuildProject(cfg); err == nil {
				t.Fatal("BuildProject accepted unsafe configuration")
			}
		})
	}
}

func TestBuildProjectDefaults(t *testing.T) {
	result, err := BuildProject(ComposeConfig{
		Name:         "My App",
		Modules:      []string{"auth_management"},
		Dependencies: testProjectDependencies(),
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
	result := generateMainGo("testapp", "Test application", []string{"auth_management", "user_management"}, ImportProfile{})

	if !strings.Contains(result, "package main") {
		t.Error("missing package main")
	}
	if !strings.Contains(result, `moduleregistry.BundleForModules(`) {
		t.Error("missing canonical bundle-backed module selector")
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
	for _, canonical := range []string{"var version = \"dev\"", "module.NewCatalog().Add(bundle).Build()", "application.WithVersion(version)", "application.WithInfrastructureProvider(infrastructure.ModuleFromConfig)", "application.WithModules(selected...)"} {
		if !strings.Contains(result, canonical) {
			t.Errorf("main.go missing canonical composition %q", canonical)
		}
	}
	for _, retired := range []string{"NewModuleSet", "module.All()", "moduleSet.Register()", "providers/zap", "viperconfig"} {
		if strings.Contains(result, retired) {
			t.Errorf("main.go contains retired composition %q", retired)
		}
	}
}

func TestGenerateMainGoAppliesImportProfile(t *testing.T) {
	profile := ImportProfile{
		BackendKit:      "github.com/acme/platformkit-backend-kit",
		BusinessModules: "github.com/acme/platformkit-business-modules",
	}

	mainGo := generateMainGo("testapp", "Test application", []string{"auth_management"}, profile)
	if !strings.Contains(mainGo, "github.com/acme/platformkit-backend-kit/app/application") {
		t.Fatal("main.go did not use profiled backend import root")
	}
	if !strings.Contains(mainGo, `"github.com/acme/platformkit-business-modules/catalog/moduleregistry"`) {
		t.Fatal("main.go did not use profiled business modules import root")
	}
	if strings.Contains(mainGo, "example.com/platformkit/") {
		t.Fatal("main.go still contains neutral import roots")
	}
}

func TestGenerateConfigYAML(t *testing.T) {
	result := generateConfigYAML("testapp", "A test app", []string{"auth_management"}, "4173")

	if !strings.Contains(result, "# testapp Configuration") {
		t.Error("missing config header")
	}
	if !strings.Contains(result, "dsn:") {
		t.Error("missing database DSN")
	}
	if !strings.Contains(result, "auth_management:") {
		t.Error("missing module config")
	}
	if !strings.Contains(result, `port: "4173"`) {
		t.Error("missing configured HTTP port")
	}
	if strings.Contains(result, "change-me") || !strings.Contains(result, "PAAS_AUTH_JWT_SECRET_KEY") {
		t.Error("config must require secret injection without a placeholder secret")
	}
}

func TestGenerateGoMod(t *testing.T) {
	remote := ProjectDependencies{
		BackendKit:      GoModuleSource{Version: "v1.2.3"},
		BusinessModules: GoModuleSource{Version: "v2.3.4"},
	}
	result := generateGoMod("github.com/myorg/testapp", remote, ImportProfile{})

	if !strings.Contains(result, "module github.com/myorg/testapp") {
		t.Error("missing module declaration")
	}
	if strings.Contains(result, "replace") {
		t.Error("should not have replace directives without explicit local sources")
	}
	if !strings.Contains(result, "go 1.26\n") {
		t.Error("go.mod does not declare the Go 1.26 baseline")
	}

	resultLocal := generateGoMod("github.com/myorg/testapp", testProjectDependencies(), ImportProfile{})
	if !strings.Contains(resultLocal, "replace") {
		t.Error("should have explicit replacement directives for local sources")
	}
}

func TestGenerateGoModAppliesImportProfile(t *testing.T) {
	profile := ImportProfile{
		BackendKit:      "github.com/acme/platformkit-backend-kit",
		BusinessModules: "github.com/acme/platformkit-business-modules",
	}
	dependencies := ProjectDependencies{
		BackendKit:      GoModuleSource{Version: "v0.0.0", ReplacePath: "../platformkit-backend-kit"},
		BusinessModules: GoModuleSource{Version: "v0.0.0", ReplacePath: "../platformkit-business-modules"},
		AdditionalReplacements: []GoModuleReplacement{
			{ModulePath: "github.com/acme/platformkit-ports", LocalPath: "../platformkit-ports"},
		},
	}

	result := generateGoMod("github.com/myorg/testapp", dependencies, profile)
	for _, needle := range []string{
		"github.com/acme/platformkit-backend-kit v0.0.0",
		"github.com/acme/platformkit-business-modules v0.0.0",
		`github.com/acme/platformkit-backend-kit => "../platformkit-backend-kit"`,
		`github.com/acme/platformkit-business-modules => "../platformkit-business-modules"`,
		`github.com/acme/platformkit-ports => "../platformkit-ports"`,
	} {
		if !strings.Contains(result, needle) {
			t.Fatalf("profiled go.mod missing %q", needle)
		}
	}
}

func testProjectDependencies() ProjectDependencies {
	return ProjectDependencies{
		BackendKit:      GoModuleSource{Version: "v0.0.0", ReplacePath: "../backend-kit"},
		BusinessModules: GoModuleSource{Version: "v0.0.0", ReplacePath: "../business-modules"},
	}
}

func TestGenerateDockerfile(t *testing.T) {
	result := generateDockerfile("testapp", "8080")

	if !strings.Contains(result, "FROM golang:1.26-alpine") {
		t.Error("missing builder stage")
	}
	if !strings.Contains(result, "EXPOSE 8080") {
		t.Error("missing port expose")
	}
	if !strings.Contains(result, "COPY locales/ ./locales/") {
		t.Error("missing locale catalog copy")
	}
	if !strings.Contains(result, `ARG VERSION=dev`) || !strings.Contains(result, `-X main.version=${VERSION}`) {
		t.Error("Dockerfile does not stamp the generated main.version symbol")
	}
	for _, vendoredBuild := range []string{"COPY vendor/ ./vendor/", "go build -mod=vendor"} {
		if !strings.Contains(result, vendoredBuild) {
			t.Errorf("Dockerfile missing reproducible local-dependency build contract %q", vendoredBuild)
		}
	}
	if strings.Contains(result, "COPY locales/ ./locales/ 2>/dev/null || true") {
		t.Error("Dockerfile contains shell syntax in a COPY instruction")
	}
	for _, absent := range []string{"COPY go.mod go.sum", "COPY migrations/"} {
		if strings.Contains(result, absent) {
			t.Errorf("Dockerfile references an ungenerated path %q", absent)
		}
	}
}

func TestGenerateDockerCompose(t *testing.T) {
	result := generateDockerCompose("testapp", "8080", "5432")

	if !strings.Contains(result, "postgres:17-alpine") {
		t.Error("missing postgres service")
	}
	if !strings.Contains(result, "redis:7-alpine") {
		t.Error("missing redis service")
	}
	if !strings.Contains(result, "nats:2-alpine") {
		t.Error("missing nats service")
	}
	for _, required := range []string{"${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}", "${PAAS_AUTH_JWT_SECRET_KEY:?set PAAS_AUTH_JWT_SECRET_KEY}", "PAAS_DATABASE_DSN"} {
		if !strings.Contains(result, required) {
			t.Errorf("docker compose missing required secret/config contract %q", required)
		}
	}
	if strings.Contains(result, "POSTGRES_PASSWORD: postgres") {
		t.Error("docker compose contains a default database password")
	}
}

func TestGenerateMakefile(t *testing.T) {
	result := generateMakefile("testapp")

	if !strings.Contains(result, "APP_NAME := testapp") {
		t.Error("missing app name")
	}
	if !strings.Contains(result, "docker-up:") {
		t.Error("missing docker-up target")
	}
	for _, vendorContract := range []string{"vendor: tidy", "mod vendor", "docker-up: vendor", "docker compose up -d --build"} {
		if !strings.Contains(result, vendorContract) {
			t.Errorf("Makefile missing vendored container contract %q", vendorContract)
		}
	}
	for _, retired := range []string{"migrate:", "migrations/seed.sql", "psql"} {
		if strings.Contains(result, retired) {
			t.Errorf("Makefile contains retired seed path %q", retired)
		}
	}
}

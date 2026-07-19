// Implements: REQ-016.
// Per: ADR-0061.
// Discipline: C-14.

package scaffold

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	modmodule "golang.org/x/mod/module"
)

var (
	composeAppNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9 _-]*[A-Za-z0-9])?$`)
	composeModulePattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
)

// GoModuleSource identifies one direct generated-project dependency. Version
// is always explicit; ReplacePath selects a caller-owned local checkout.
type GoModuleSource struct {
	Version     string `json:"version"`
	ReplacePath string `json:"replacePath,omitempty"`
}

// GoModuleReplacement makes a transitive workspace module available to a
// generated project without relying on ambient go.work state.
type GoModuleReplacement struct {
	ModulePath string `json:"modulePath"`
	LocalPath  string `json:"localPath"`
}

// ProjectDependencies is the complete, reproducible Go dependency contract
// for a generated project.
type ProjectDependencies struct {
	BackendKit             GoModuleSource        `json:"backendKit"`
	BusinessModules        GoModuleSource        `json:"businessModules"`
	AdditionalReplacements []GoModuleReplacement `json:"additionalReplacements,omitempty"`
}

// ComposeConfig holds parameters for project file generation.
type ComposeConfig struct {
	Name         string              // Application name
	ModulePath   string              // Go module path (e.g. "github.com/myorg/myapp")
	Description  string              // Application description
	Modules      []string            // Module names to include
	Port         string              // Application port (default: "8080")
	DBPort       string              // PostgreSQL port (default: "5432")
	Dependencies ProjectDependencies // Explicit versions and optional local checkouts

	ImportProfile ImportProfile // Repository roots used by generated Go source
}

// ProjectResult holds all generated files for a complete project.
type ProjectResult struct {
	Name               string          `json:"name"`
	ModulePath         string          `json:"modulePath"`
	Modules            []string        `json:"modules"`
	Files              []GeneratedFile `json:"files"`
	DirectoryStructure string          `json:"directoryStructure"`
	DBName             string          `json:"dbName"`
	QuickStart         string          `json:"quickStart"`
	NextSteps          []string        `json:"nextSteps"`
}

// BuildProject generates all files for a complete, buildable platformkit project.
func BuildProject(cfg ComposeConfig) (ProjectResult, error) {
	applyComposeDefaults(&cfg)
	if err := validateComposeConfig(cfg); err != nil {
		return ProjectResult{}, err
	}
	projectManifest, err := generateProjectManifest(cfg)
	if err != nil {
		return ProjectResult{}, err
	}

	dbName := strings.ReplaceAll(strings.ToLower(cfg.Name), " ", "_")

	files := []GeneratedFile{
		{Path: ".platformkit/project.json", Content: projectManifest},
		{Path: ".env.example", Content: generateEnvExample(dbName)},
		{Path: ".gitignore", Content: generateGitIgnore()},
		{Path: ".dockerignore", Content: generateDockerIgnore()},
		{Path: "main.go", Content: generateMainGo(cfg.Name, cfg.Description, cfg.Modules, cfg.ImportProfile)},
		{Path: "go.mod", Content: generateGoMod(cfg.ModulePath, cfg.Dependencies, cfg.ImportProfile)},
		{Path: "config.yaml", Content: generateConfigYAML(cfg.Name, cfg.Description, cfg.Modules, cfg.Port)},
		{Path: "Dockerfile", Content: generateDockerfile(cfg.Name, cfg.Port)},
		{Path: "docker-compose.yml", Content: generateDockerCompose(cfg.Name, cfg.Port, cfg.DBPort)},
		{Path: "Makefile", Content: generateMakefile(cfg.Name)},
		{Path: "locales/en.json", Content: "[]\n"},
	}

	directoryStructure := fmt.Sprintf(`%s/
├── .platformkit/
│   └── project.json        # Platformkit app manifest
├── .env.example            # Required local secret names (values stay uncommitted)
├── .gitignore             # Secret and build-artifact exclusions
├── .dockerignore          # Minimal, secret-safe container context
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── config.yaml             # Application configuration
├── Dockerfile              # Multi-stage Docker build
├── docker-compose.yml      # Local development stack
├── Makefile                # Build and dev commands
└── locales/                # i18n translation files
    └── en.json              # Initial English message catalog
`, cfg.Name)

	return ProjectResult{
		Name:               cfg.Name,
		ModulePath:         cfg.ModulePath,
		Modules:            append([]string(nil), cfg.Modules...),
		Files:              files,
		DirectoryStructure: directoryStructure,
		DBName:             dbName,
		QuickStart: fmt.Sprintf(`# Quick Start
1. Write files to disk and cd %s
2. Copy .env.example to .env and set every secret value
3. make deps       # Download dependencies
4. make docker-up  # Start the application and its dependencies
5. make docker-logs
`, cfg.Name),
		NextSteps: []string{"scaffold.WriteFiles", "runtime.Build", "runtime.Migrate"},
	}, nil
}

func validateComposeConfig(cfg ComposeConfig) error {
	if !composeAppNamePattern.MatchString(cfg.Name) {
		return fmt.Errorf("application name must start with a letter or digit and contain only letters, digits, spaces, underscores, or hyphens")
	}
	if err := validateComposeText("description", cfg.Description); err != nil {
		return err
	}
	if err := validateGoModulePath(cfg.ModulePath); err != nil {
		return fmt.Errorf("project module path: %w", err)
	}
	if err := validateComposePort("application port", cfg.Port); err != nil {
		return err
	}
	if err := validateComposePort("database port", cfg.DBPort); err != nil {
		return err
	}
	if err := validateProjectDependencies(cfg.Dependencies, cfg.ImportProfile); err != nil {
		return err
	}
	seenModules := make(map[string]struct{}, len(cfg.Modules))
	if len(cfg.Modules) == 0 {
		return fmt.Errorf("at least one module is required")
	}
	for _, moduleName := range cfg.Modules {
		if !composeModulePattern.MatchString(moduleName) {
			return fmt.Errorf("module name %q must use canonical snake_case", moduleName)
		}
		if _, duplicate := seenModules[moduleName]; duplicate {
			return fmt.Errorf("module name %q is duplicated", moduleName)
		}
		if moduleName == "infrastructure" {
			return fmt.Errorf("module name %q is application-owned; infrastructure is wired through ModuleFromConfig", moduleName)
		}
		seenModules[moduleName] = struct{}{}
	}
	return nil
}

func validateProjectDependencies(deps ProjectDependencies, profile ImportProfile) error {
	profile = profile.normalized()
	direct := []struct {
		name       string
		modulePath string
		source     GoModuleSource
	}{
		{name: "backend kit", modulePath: profile.BackendKit, source: deps.BackendKit},
		{name: "business modules", modulePath: profile.BusinessModules, source: deps.BusinessModules},
	}
	seen := make(map[string]struct{}, len(deps.AdditionalReplacements)+len(direct))
	for _, dependency := range direct {
		if err := validateGoModulePath(dependency.modulePath); err != nil {
			return fmt.Errorf("%s module path: %w", dependency.name, err)
		}
		canonicalVersion := modmodule.CanonicalVersion(dependency.source.Version)
		if canonicalVersion == "" || canonicalVersion != dependency.source.Version {
			return fmt.Errorf("%s version %q must be an explicit canonical Go module version", dependency.name, dependency.source.Version)
		}
		if err := modmodule.Check(dependency.modulePath, dependency.source.Version); err != nil {
			return fmt.Errorf("%s module %s@%s is invalid: %w", dependency.name, dependency.modulePath, dependency.source.Version, err)
		}
		if dependency.source.ReplacePath != "" {
			if err := validateLocalReplacementPath(dependency.source.ReplacePath); err != nil {
				return fmt.Errorf("%s replacement: %w", dependency.name, err)
			}
		}
		seen[dependency.modulePath] = struct{}{}
	}
	for _, replacement := range deps.AdditionalReplacements {
		if err := validateGoModulePath(replacement.ModulePath); err != nil {
			return fmt.Errorf("additional replacement module path: %w", err)
		}
		if _, duplicate := seen[replacement.ModulePath]; duplicate {
			return fmt.Errorf("Go module replacement %q is duplicated", replacement.ModulePath)
		}
		if err := validateLocalReplacementPath(replacement.LocalPath); err != nil {
			return fmt.Errorf("replacement for %s: %w", replacement.ModulePath, err)
		}
		seen[replacement.ModulePath] = struct{}{}
	}
	return nil
}

func validateGoModulePath(value string) error {
	if err := modmodule.CheckPath(value); err != nil {
		return fmt.Errorf("%q is not a canonical Go module path: %w", value, err)
	}
	return nil
}

func validateLocalReplacementPath(value string) error {
	if value == "" {
		return fmt.Errorf("local path is required")
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("local path must be a single line without NUL bytes")
	}
	if filepath.Clean(value) != value || value == "." {
		return fmt.Errorf("local path %q must be canonical and must not resolve to the generated project", value)
	}
	return nil
}

func validateComposeText(field, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%s must be a single line without NUL bytes", field)
	}
	return nil
}

func validateComposePort(field, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != value {
		return fmt.Errorf("%s %q must be a canonical integer between 1 and 65535", field, value)
	}
	return nil
}

func applyComposeDefaults(cfg *ComposeConfig) {
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DBPort == "" {
		cfg.DBPort = "5432"
	}
	if cfg.ModulePath == "" {
		cfg.ModulePath = "github.com/myorg/" + strings.ReplaceAll(strings.ToLower(cfg.Name), " ", "-")
	}
}

func generateEnvExample(dbName string) string {
	return fmt.Sprintf(`POSTGRES_USER=postgres
POSTGRES_DB=%s
POSTGRES_PASSWORD=
PAAS_DATABASE_DSN=
PAAS_REDIS_ADDR=127.0.0.1:6379
PAAS_NATS_URL=nats://127.0.0.1:4222
PAAS_AUTH_JWT_SECRET_KEY=
PAAS_NATS_CONTEXT_AUTH_SECRET=
`, dbName)
}

func generateGitIgnore() string {
	return `.env
bin/
coverage.out
coverage.html
vendor/
`
}

func generateDockerIgnore() string {
	return `.git
.env
bin
coverage.out
coverage.html
`
}

func generateProjectManifest(cfg ComposeConfig) (string, error) {
	manifest := struct {
		SchemaVersion int      `json:"schemaVersion"`
		Kind          string   `json:"kind"`
		CreatedBy     string   `json:"createdBy"`
		Name          string   `json:"name"`
		ModulePath    string   `json:"modulePath"`
		Modules       []string `json:"modules"`
		Port          string   `json:"port"`
	}{
		SchemaVersion: 1,
		Kind:          "platformkit-app",
		CreatedBy:     "platformkit init",
		Name:          cfg.Name,
		ModulePath:    cfg.ModulePath,
		Modules:       append([]string(nil), cfg.Modules...),
		Port:          cfg.Port,
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal project manifest: %w", err)
	}
	return string(body) + "\n", nil
}

func generateConfigYAML(name, description string, modules []string, port string) string {
	dbName := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	mcpName := strings.ReplaceAll(strings.ToLower(name), " ", "-")

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s Configuration\n", name)
	if description != "" {
		fmt.Fprintf(&sb, "# %s\n", description)
	}
	sb.WriteString("\n# Server configuration\n")
	sb.WriteString("server:\n")
	sb.WriteString("  host: \"0.0.0.0\"\n")
	fmt.Fprintf(&sb, "  port: %q\n", port)
	sb.WriteString("  metrics_port: \"9091\"\n")
	sb.WriteString("  read_timeout: 30s\n")
	sb.WriteString("  write_timeout: 30s\n")
	sb.WriteString("  idle_timeout: 120s\n")
	sb.WriteString("  shutdown_timeout: 10s\n")

	sb.WriteString("\n# Database configuration\n")
	sb.WriteString("database:\n")
	sb.WriteString("  dsn: \"\"  # Required via PAAS_DATABASE_DSN\n")
	sb.WriteString("  driver: \"postgres\"\n")
	sb.WriteString("  maxOpenConns: 25\n")
	sb.WriteString("  maxIdleConns: 5\n")
	sb.WriteString("  connMaxLifetimeMinutes: 5\n")
	sb.WriteString("  logLevel: \"info\"\n")

	sb.WriteString("\n# Redis configuration\n")
	sb.WriteString("redis:\n")
	sb.WriteString("  addr: \"127.0.0.1:6379\"\n")
	sb.WriteString("  password: \"\"\n")
	sb.WriteString("  db: 0\n")

	sb.WriteString("\n# NATS configuration\n")
	sb.WriteString("nats:\n")
	sb.WriteString("  url: \"nats://localhost:4222\"\n")
	sb.WriteString("  context_auth_secret: \"\"\n")
	fmt.Fprintf(&sb, "  client_id: \"%s\"\n", dbName)

	sb.WriteString("\n# Logging configuration\n")
	sb.WriteString("logging:\n")
	sb.WriteString("  log_level: \"info\"\n")
	sb.WriteString("  env: \"development\"\n")

	sb.WriteString("\n# Authentication configuration\n")
	sb.WriteString("auth:\n")
	sb.WriteString("  jwt_secret_key: \"\"  # Required via PAAS_AUTH_JWT_SECRET_KEY (minimum 32 characters)\n")
	sb.WriteString("  token_expiry: \"24h\"\n")
	sb.WriteString("  refresh_token_expiry: \"168h\"\n")

	sb.WriteString("\n# Cache configuration\n")
	sb.WriteString("cache:\n")
	sb.WriteString("  default_ttl: \"1h\"\n")
	sb.WriteString("  max_entries: 10000\n")

	sb.WriteString("\n# Localizer configuration\n")
	sb.WriteString("localizer:\n")
	sb.WriteString("  langDir: \"./locales\"\n")
	sb.WriteString("  fallbackLang: \"en\"\n")
	sb.WriteString("  type: \"goi18n\"\n")

	sb.WriteString("\n# Feature flags\n")
	sb.WriteString("features:\n")
	sb.WriteString("  enable_metrics: true\n")
	sb.WriteString("  enable_tracing: false\n")

	sb.WriteString("\n# MCP configuration\n")
	sb.WriteString("mcp:\n")
	sb.WriteString("  enabled: true\n")
	sb.WriteString("  auth_enabled: true\n")
	sb.WriteString("  required_scopes:\n")
	sb.WriteString("    - \"mcp:tools\"\n")
	sb.WriteString("  port: \"9999\"\n")
	sb.WriteString("  host: \"0.0.0.0\"\n")
	sb.WriteString("  protocol: \"http\"\n")
	fmt.Fprintf(&sb, "  name: \"%s-mcp\"\n", mcpName)

	sb.WriteString("\n# Modules configuration\n")
	sb.WriteString("modules:\n")
	for _, mod := range modules {
		fmt.Fprintf(&sb, "  %s:\n", mod)
		sb.WriteString("    enabled: true\n")
	}
	return sb.String()
}

func generateMainGo(name, description string, modules []string, profile ImportProfile) string {
	var sb strings.Builder
	sb.WriteString("// Implements: REQ-016.\n")
	sb.WriteString("// Per: ADR-0061.\n")
	sb.WriteString("// Discipline: C-14.\n\n")
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"fmt\"\n")
	sb.WriteString("\t\"os\"\n\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/app/application\"\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/app/module\"\n")
	sb.WriteString("\t_ \"example.com/platformkit/backend-kit/app/event/providers/jetstream\"\n\n")
	sb.WriteString("\t\"example.com/platformkit/business-modules/catalog/moduleregistry\"\n")
	sb.WriteString("\t\"example.com/platformkit/business-modules/infrastructure\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString("var version = \"dev\"\n\n")

	sb.WriteString("func main() {\n")
	sb.WriteString("\tif err := run(); err != nil {\n")
	sb.WriteString("\t\tfmt.Fprintf(os.Stderr, \"%s: %v\\n\", os.Args[0], err)\n")
	sb.WriteString("\t\tos.Exit(1)\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("func run() error {\n")
	sb.WriteString("\tbundle, err := moduleregistry.BundleForModules(\n")
	for _, mod := range modules {
		fmt.Fprintf(&sb, "\t\t%q,\n", mod)
	}
	sb.WriteString("\t)\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\treturn fmt.Errorf(\"select module bundle: %w\", err)\n")
	sb.WriteString("\t}\n\n")
	sb.WriteString("\tcatalog, err := module.NewCatalog().Add(bundle).Build()\n")
	sb.WriteString("\tif err != nil {\n")
	sb.WriteString("\t\treturn fmt.Errorf(\"build module catalog: %w\", err)\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\tselected := make([]module.Module, 0, len(bundle.Defaults()))\n")
	sb.WriteString("\tfor _, id := range bundle.Defaults() {\n")
	sb.WriteString("\t\tmod, err := catalog.BuildModule(id)\n")
	sb.WriteString("\t\tif err != nil {\n")
	sb.WriteString("\t\t\treturn fmt.Errorf(\"build module %q: %w\", id, err)\n")
	sb.WriteString("\t\t}\n")
	sb.WriteString("\t\tselected = append(selected, mod)\n")
	sb.WriteString("\t}\n\n")

	sb.WriteString("\tapp := application.New(\n")
	fmt.Fprintf(&sb, "\t\tapplication.WithName(%q),\n", name)
	fmt.Fprintf(&sb, "\t\tapplication.WithDescription(%q),\n", description)
	sb.WriteString("\t\tapplication.WithVersion(version),\n")
	sb.WriteString("\t\tapplication.WithInfrastructureProvider(infrastructure.ModuleFromConfig),\n")
	sb.WriteString("\t\tapplication.WithModules(selected...),\n")
	sb.WriteString("\t)\n\n")
	sb.WriteString("\treturn app.Run()\n")
	sb.WriteString("}\n")
	return applyImportProfile(sb.String(), profile)
}

func generateGoMod(modulePath string, deps ProjectDependencies, profile ImportProfile) string {
	profile = profile.normalized()
	var sb strings.Builder
	fmt.Fprintf(&sb, "module %s\n\n", modulePath)
	sb.WriteString("go 1.26\n\n")
	sb.WriteString("require (\n")
	fmt.Fprintf(&sb, "\t%s %s\n", profile.BackendKit, deps.BackendKit.Version)
	fmt.Fprintf(&sb, "\t%s %s\n", profile.BusinessModules, deps.BusinessModules.Version)
	sb.WriteString(")\n")

	replacements := make([]GoModuleReplacement, 0, len(deps.AdditionalReplacements)+2)
	if deps.BackendKit.ReplacePath != "" {
		replacements = append(replacements, GoModuleReplacement{ModulePath: profile.BackendKit, LocalPath: deps.BackendKit.ReplacePath})
	}
	if deps.BusinessModules.ReplacePath != "" {
		replacements = append(replacements, GoModuleReplacement{ModulePath: profile.BusinessModules, LocalPath: deps.BusinessModules.ReplacePath})
	}
	replacements = append(replacements, deps.AdditionalReplacements...)
	if len(replacements) > 0 {
		sort.Slice(replacements, func(i, j int) bool { return replacements[i].ModulePath < replacements[j].ModulePath })
		sb.WriteString("\n// Explicit source graph for this generated project.\n")
		sb.WriteString("replace (\n")
		for _, replacement := range replacements {
			fmt.Fprintf(&sb, "\t%s => %s\n", replacement.ModulePath, strconv.Quote(replacement.LocalPath))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}

func generateDockerfile(name, port string) string {
	binName := strings.ReplaceAll(strings.ToLower(name), " ", "-")
	return fmt.Sprintf(`# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace
COPY go.mod ./
COPY vendor/ ./vendor/

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app/%s main.go

# Runtime stage
FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /app/%s .
COPY config.yaml .
COPY locales/ ./locales/

RUN mkdir -p /var/log/app /var/cache/app && chown -R app:app /app /var/log/app /var/cache/app

USER app

EXPOSE %s 9091

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:%s/health || exit 1

CMD ["./%s", "--config", "config.yaml"]
`, binName, binName, port, port, binName)
}

func generateDockerCompose(name, port, dbPort string) string {
	dbName := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	svcName := strings.ReplaceAll(strings.ToLower(name), " ", "-")

	return fmt.Sprintf(`services:
  app:
    build: .
    container_name: %s
    ports:
      - "%s:%s"
      - "9091:9091"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      nats:
        condition: service_started
    environment:
      PAAS_DATABASE_DSN: postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-%s}?sslmode=disable
      PAAS_REDIS_ADDR: redis:6379
      PAAS_NATS_URL: nats://nats:4222
      PAAS_AUTH_JWT_SECRET_KEY: ${PAAS_AUTH_JWT_SECRET_KEY:?set PAAS_AUTH_JWT_SECRET_KEY}
      PAAS_NATS_CONTEXT_AUTH_SECRET: ${PAAS_NATS_CONTEXT_AUTH_SECRET:?set PAAS_NATS_CONTEXT_AUTH_SECRET}
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./locales:/app/locales:ro
    restart: unless-stopped

  postgres:
    image: postgres:17-alpine
    container_name: %s-postgres
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-%s}
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
    ports:
      - "%s:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: %s-redis
    ports:
      - "6379:6379"
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  nats:
    image: nats:2-alpine
    container_name: %s-nats
    ports:
      - "4222:4222"
      - "8222:8222"
    command: ["-js", "-m", "8222"]

volumes:
  pgdata:
  redisdata:
`, svcName, port, port, dbName,
		svcName, dbName, dbPort,
		svcName, svcName)
}

func generateMakefile(name string) string {
	binName := strings.ReplaceAll(strings.ToLower(name), " ", "-")

	return fmt.Sprintf(`.PHONY: all build run test lint fmt tidy deps vendor docker-build docker-up docker-down dev clean

# Application
APP_NAME := %s
BINARY := bin/$(APP_NAME)

# Go
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags="-s -w"

all: lint test build

## Build

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) main.go

run: build
	@test -f .env || (echo ".env is required; copy .env.example and set its values" && exit 1)
	@set -a; . ./.env; set +a; ./$(BINARY) --config config.yaml

clean:
	rm -rf bin/ coverage.out coverage.html

## Development

dev:
	@test -f .env || (echo ".env is required; copy .env.example and set its values" && exit 1)
	@set -a; . ./.env; set +a; $(GO) run main.go --config config.yaml

deps:
	$(GO) mod tidy

tidy:
	$(GO) mod tidy

vendor: tidy
	$(GO) mod vendor

fmt:
	gofumpt -l -w .
	goimports -w .

## Testing

test:
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## Linting

lint: fmt tidy
	$(GO) vet ./...

## Docker

docker-build: vendor
	docker build -t $(APP_NAME) .

docker-up: vendor
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app
`, binName)
}

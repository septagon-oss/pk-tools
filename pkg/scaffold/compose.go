package scaffold

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ComposeConfig holds parameters for project file generation.
type ComposeConfig struct {
	Name         string   // Application name
	ModulePath   string   // Go module path (e.g. "github.com/myorg/myapp")
	Description  string   // Application description
	Modules      []string // Module names to include
	Port         string   // Application port (default: "8080")
	DBPort       string   // PostgreSQL port (default: "5432")
	AdminEmail   string   // Admin email for seed data (default: "admin@example.com")
	TenantName   string   // Default tenant name for seed data (default: "Default")
	LocalDevMode bool     // Include go.mod replace directives for local dev

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
	projectManifest, err := GenerateProjectManifest(cfg)
	if err != nil {
		return ProjectResult{}, err
	}

	dbName := strings.ReplaceAll(strings.ToLower(cfg.Name), " ", "_")

	files := []GeneratedFile{
		{Path: ".platformkit/project.json", Content: projectManifest},
		{Path: "main.go", Content: GenerateMainGoWithProfile(cfg.Name, cfg.Modules, cfg.Port, cfg.ImportProfile)},
		{Path: "go.mod", Content: GenerateGoModWithProfile(cfg.ModulePath, cfg.LocalDevMode, cfg.ImportProfile)},
		{Path: "config.yaml", Content: GenerateConfigYAML(cfg.Name, cfg.Description, cfg.Modules)},
		{Path: "Dockerfile", Content: GenerateDockerfile(cfg.Name, cfg.Port)},
		{Path: "docker-compose.yml", Content: GenerateDockerCompose(cfg.Name, cfg.Port, cfg.DBPort)},
		{Path: "Makefile", Content: GenerateMakefile(cfg.Name)},
		{Path: "migrations/seed.sql", Content: GenerateSeedData(cfg.AdminEmail, cfg.TenantName, cfg.Modules)},
	}

	directoryStructure := fmt.Sprintf(`%s/
├── .platformkit/
│   └── project.json        # Platformkit app manifest
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── config.yaml             # Application configuration
├── Dockerfile              # Multi-stage Docker build
├── docker-compose.yml      # Local development stack
├── Makefile                # Build and dev commands
├── migrations/
│   └── seed.sql            # Initial seed data
├── locales/                # i18n translation files
│   └── en.json
└── internal/               # Application-specific code
    └── README.md
`, cfg.Name)

	return ProjectResult{
		Name:               cfg.Name,
		ModulePath:         cfg.ModulePath,
		Modules:            cfg.Modules,
		Files:              files,
		DirectoryStructure: directoryStructure,
		DBName:             dbName,
		QuickStart: fmt.Sprintf(`# Quick Start
1. Write files to disk
2. cd %s
3. make deps       # Download dependencies
4. make docker-up  # Start PostgreSQL, Redis, NATS
5. make migrate    # Run seed migrations
6. make run        # Start the application
`, cfg.Name),
		NextSteps: []string{"scaffold.WriteFiles", "runtime.Build", "runtime.Migrate"},
	}, nil
}

func applyComposeDefaults(cfg *ComposeConfig) {
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DBPort == "" {
		cfg.DBPort = "5432"
	}
	if cfg.AdminEmail == "" {
		cfg.AdminEmail = "admin@example.com"
	}
	if cfg.TenantName == "" {
		cfg.TenantName = "Default"
	}
	if cfg.ModulePath == "" {
		cfg.ModulePath = "github.com/myorg/" + strings.ReplaceAll(strings.ToLower(cfg.Name), " ", "-")
	}
}

// GenerateProjectManifest produces the app marker used by CLI context detection.
func GenerateProjectManifest(cfg ComposeConfig) (string, error) {
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

// GenerateConfigYAML produces a config.yaml for a platformkit application.
func GenerateConfigYAML(name, description string, modules []string) string {
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
	sb.WriteString("  port: \"8080\"\n")
	sb.WriteString("  metrics_port: \"9091\"\n")
	sb.WriteString("  read_timeout: 30s\n")
	sb.WriteString("  write_timeout: 30s\n")
	sb.WriteString("  idle_timeout: 120s\n")
	sb.WriteString("  shutdown_timeout: 10s\n")

	sb.WriteString("\n# Database configuration\n")
	sb.WriteString("database:\n")
	fmt.Fprintf(&sb, "  dsn: \"postgres://postgres:postgres@localhost:5432/%s?sslmode=disable\"\n", dbName)
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
	sb.WriteString("  jwt_secret_key: \"change-me-in-production\"\n")
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

// GenerateMainGo produces a main.go entry point for a platformkit application.
func GenerateMainGo(name string, modules []string, port string) string {
	return GenerateMainGoWithProfile(name, modules, port, ImportProfile{})
}

// GenerateMainGoWithProfile produces a main.go entry point using the supplied
// repository roots for PlatformKit imports.
func GenerateMainGoWithProfile(name string, modules []string, port string, profile ImportProfile) string {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"flag\"\n")
	sb.WriteString("\t\"fmt\"\n")
	sb.WriteString("\t\"os\"\n\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/app/application\"\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/app/module\"\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/infrastructure/config\"\n")
	sb.WriteString("\tviperconfig \"example.com/platformkit/backend-kit/infrastructure/config/providers/viper\"\n")
	sb.WriteString("\t\"example.com/platformkit/backend-kit/observability/logger/providers/zap\"\n\n")
	sb.WriteString("\t// Import NATS provider\n")
	sb.WriteString("\t_ \"example.com/platformkit/backend-kit/app/event/providers/nats\"\n\n")
	sb.WriteString("\t// Import platform modules\n")
	sb.WriteString("\tplatformmodules \"example.com/platformkit/business-modules\"\n")
	sb.WriteString(")\n\n")

	sb.WriteString("func main() {\n")
	sb.WriteString("\tif err := run(); err != nil {\n")
	sb.WriteString("\t\tfmt.Fprintf(os.Stderr, \"%s: %v\\n\", os.Args[0], err)\n")
	sb.WriteString("\t\tos.Exit(1)\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n\n")

	sb.WriteString("func run() error {\n")
	sb.WriteString("\t// Parse config file from --config flag\n")
	sb.WriteString("\tconfigFile := flag.String(\"config\", \"\", \"Path to configuration file\")\n")
	sb.WriteString("\tflag.Parse()\n\n")

	sb.WriteString("\t// Load configuration\n")
	sb.WriteString("\tvar cfg *config.Config\n")
	sb.WriteString("\tif *configFile != \"\" {\n")
	sb.WriteString("\t\tvar err error\n")
	sb.WriteString("\t\tcfg, err = viperconfig.LoadConfig(*configFile)\n")
	sb.WriteString("\t\tif err != nil {\n")
	sb.WriteString("\t\t\treturn fmt.Errorf(\"failed to load config: %w\", err)\n")
	sb.WriteString("\t\t}\n")
	sb.WriteString("\t} else {\n")
	sb.WriteString("\t\tcfg = &config.Config{Logging: config.LoggingConfig{LogLevel: \"info\", Env: \"development\"}}\n")
	sb.WriteString("\t}\n\n")

	sb.WriteString("\tif _, err := zap.NewLogger(cfg); err != nil {\n")
	sb.WriteString("\t\treturn fmt.Errorf(\"failed to initialize logger: %w\", err)\n")
	sb.WriteString("\t}\n\n")

	sb.WriteString("\t// Select modules\n")
	sb.WriteString("\tmoduleSet := platformmodules.NewModuleSet().WithModules(\n")
	for _, mod := range modules {
		fmt.Fprintf(&sb, "\t\t%q,\n", mod)
	}
	sb.WriteString("\t)\n")
	sb.WriteString("\n")

	sb.WriteString("\tif err := moduleSet.Register(); err != nil {\n")
	sb.WriteString("\t\treturn fmt.Errorf(\"failed to register modules: %w\", err)\n")
	sb.WriteString("\t}\n\n")

	sb.WriteString("\tmodules := module.All()\n\n")

	sb.WriteString("\tapp := application.New(\n")
	fmt.Fprintf(&sb, "\t\tapplication.WithName(%q),\n", name)
	sb.WriteString("\t\tapplication.WithVersion(\"1.0.0\"),\n")
	sb.WriteString("\t\tapplication.WithModules(modules...),\n")
	sb.WriteString("\t)\n\n")
	sb.WriteString("\treturn app.Run()\n")
	sb.WriteString("}\n")
	return applyImportProfile(sb.String(), profile)
}

// ModuleToFuncName converts a snake_case module name to a PascalCase method suffix.
// e.g. "booking_management" -> "BookingManagement"
func ModuleToFuncName(moduleName string) string {
	parts := strings.Split(moduleName, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return result.String()
}

// GenerateGoMod produces a go.mod file for a platformkit application.
func GenerateGoMod(modulePath string, localDev bool) string {
	return GenerateGoModWithProfile(modulePath, localDev, ImportProfile{})
}

// GenerateGoModWithProfile produces a go.mod file using the supplied
// repository roots and local replacement paths.
func GenerateGoModWithProfile(modulePath string, localDev bool, profile ImportProfile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "module %s\n\n", modulePath)
	sb.WriteString("go 1.24\n\n")
	sb.WriteString("require (\n")
	sb.WriteString("\texample.com/platformkit/backend-kit v0.0.0\n")
	sb.WriteString("\texample.com/platformkit/business-modules v0.0.0\n")
	sb.WriteString("\tgithub.com/danielgtaylor/huma/v2 v2.32.0\n")
	sb.WriteString("\tgithub.com/google/uuid v1.6.0\n")
	sb.WriteString("\tgithub.com/spf13/viper v1.19.0\n")
	sb.WriteString("\tgo.uber.org/fx v1.23.0\n")
	sb.WriteString("\tgo.uber.org/zap v1.27.0\n")
	sb.WriteString("\tgorm.io/gorm v1.25.12\n")
	sb.WriteString("\tgorm.io/driver/postgres v1.5.11\n")
	sb.WriteString("\tgithub.com/nats-io/nats.go v1.38.0\n")
	sb.WriteString("\tgithub.com/redis/go-redis/v9 v9.7.0\n")
	sb.WriteString(")\n")

	if localDev {
		sb.WriteString("\n// Local development: point to local copies\n")
		sb.WriteString("replace (\n")
		sb.WriteString("\texample.com/platformkit/backend-kit => ../backend-kit\n")
		sb.WriteString("\texample.com/platformkit/business-modules => ../business-modules\n")
		sb.WriteString(")\n")
	}

	return applyImportProfile(sb.String(), profile)
}

// GenerateDockerfile produces a multi-stage Dockerfile for a platformkit application.
func GenerateDockerfile(name, port string) string {
	if port == "" {
		port = "8080"
	}
	binName := strings.ReplaceAll(strings.ToLower(name), " ", "-")
	return fmt.Sprintf(`# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /app/%s main.go

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /app/%s .
COPY config.yaml .
COPY migrations/ ./migrations/
COPY locales/ ./locales/ 2>/dev/null || true

RUN mkdir -p /var/log/app /var/cache/app && chown -R app:app /app /var/log/app /var/cache/app

USER app

EXPOSE %s 9091

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:%s/health || exit 1

CMD ["./%s", "--config", "config.yaml"]
`, binName, binName, port, port, binName)
}

// GenerateDockerCompose produces a docker-compose.yml with app, PostgreSQL, Redis, and NATS.
func GenerateDockerCompose(name, port, dbPort string) string {
	if port == "" {
		port = "8080"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
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
      - DATABASE_DSN=postgres://postgres:postgres@postgres:5432/%s?sslmode=disable
      - REDIS_ADDR=redis:6379
      - NATS_URL=nats://nats:4222
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./locales:/app/locales:ro
    restart: unless-stopped

  postgres:
    image: postgres:17-alpine
    container_name: %s-postgres
    environment:
      POSTGRES_DB: %s
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "%s:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
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

// GenerateMakefile produces a Makefile with build, run, test, lint, docker, and dev targets.
func GenerateMakefile(name string) string {
	binName := strings.ReplaceAll(strings.ToLower(name), " ", "-")

	return fmt.Sprintf(`.PHONY: all build run test lint fmt tidy deps migrate docker-build docker-up docker-down dev clean

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
	./$(BINARY) --config config.yaml

clean:
	rm -rf bin/ coverage.out coverage.html

## Development

dev:
	$(GO) run main.go --config config.yaml

deps:
	$(GO) mod download
	$(GO) mod tidy

tidy:
	$(GO) mod tidy

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

## Database

migrate:
	@echo "Running seed migrations..."
	@if command -v psql > /dev/null; then \
		psql "$$(grep dsn config.yaml | head -1 | awk -F'"' '{print $$2}')" -f migrations/seed.sql; \
	else \
		echo "psql not found. Run: docker exec -i %s-postgres psql -U postgres -d %s < migrations/seed.sql"; \
	fi

## Docker

docker-build:
	docker build -t $(APP_NAME) .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app
`, binName, binName, strings.ReplaceAll(strings.ToLower(name), " ", "_"))
}

// EscapeSQLString escapes single quotes for safe SQL interpolation.
func EscapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// GenerateSeedData produces SQL seed data with tenant, roles, admin user, and module-specific data.
func GenerateSeedData(adminEmail, tenantName string, modules []string) string {
	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}
	if tenantName == "" {
		tenantName = "Default"
	}

	safeTenantName := EscapeSQLString(tenantName)
	safeAdminEmail := EscapeSQLString(adminEmail)
	safeSlug := EscapeSQLString(strings.ReplaceAll(strings.ToLower(tenantName), " ", "-"))

	var sb strings.Builder
	sb.WriteString("-- Seed data for initial setup\n")
	sb.WriteString("-- Generated by platformkit compose.GenerateSeedData\n\n")

	sb.WriteString("BEGIN;\n\n")

	// Default tenant
	sb.WriteString("-- Default tenant\n")
	sb.WriteString("INSERT INTO tenants (id, name, slug, status, created_at, updated_at)\n")
	sb.WriteString("VALUES (\n")
	sb.WriteString("    '00000000-0000-0000-0000-000000000001',\n")
	fmt.Fprintf(&sb, "    '%s',\n", safeTenantName)
	fmt.Fprintf(&sb, "    '%s',\n", safeSlug)
	sb.WriteString("    'active',\n")
	sb.WriteString("    NOW(),\n")
	sb.WriteString("    NOW()\n")
	sb.WriteString(") ON CONFLICT (id) DO NOTHING;\n\n")

	// Default roles
	sb.WriteString("-- Default roles\n")
	for _, role := range []struct{ id, name, desc string }{
		{"00000000-0000-0000-0000-000000000010", "admin", "Full system administrator"},
		{"00000000-0000-0000-0000-000000000011", "user", "Standard user"},
		{"00000000-0000-0000-0000-000000000012", "viewer", "Read-only access"},
	} {
		sb.WriteString("INSERT INTO roles (id, name, description, tenant_id, created_at, updated_at)\n")
		fmt.Fprintf(&sb, "VALUES ('%s', '%s', '%s', '00000000-0000-0000-0000-000000000001', NOW(), NOW())\n", role.id, role.name, role.desc)
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n\n")
	}

	// Default admin user (no password persisted here — the scaffold seeds the
	// users row only; credentials are provisioned separately via the auth_management
	// bootstrap, which reads the admin password from environment configuration.)
	sb.WriteString("-- Default admin user row — authentication credentials are provisioned separately\n")
	sb.WriteString("-- via the auth_management bootstrap using environment-supplied values.\n")
	sb.WriteString("INSERT INTO users (id, email, name, status, tenant_id, created_at, updated_at)\n")
	sb.WriteString("VALUES (\n")
	sb.WriteString("    '00000000-0000-0000-0000-000000000100',\n")
	fmt.Fprintf(&sb, "    '%s',\n", safeAdminEmail)
	sb.WriteString("    'Admin',\n")
	sb.WriteString("    'active',\n")
	sb.WriteString("    '00000000-0000-0000-0000-000000000001',\n")
	sb.WriteString("    NOW(),\n")
	sb.WriteString("    NOW()\n")
	sb.WriteString(") ON CONFLICT (id) DO NOTHING;\n\n")

	// Assign admin role
	sb.WriteString("-- Assign admin role to default user\n")
	sb.WriteString("INSERT INTO user_roles (user_id, role_id, created_at)\n")
	sb.WriteString("VALUES ('00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000010', NOW())\n")
	sb.WriteString("ON CONFLICT DO NOTHING;\n\n")

	// Module-specific seed data
	moduleSet := map[string]bool{}
	for _, m := range modules {
		moduleSet[m] = true
	}

	if moduleSet["billing_management"] {
		sb.WriteString("-- Billing: Default pricing plans\n")
		sb.WriteString("INSERT INTO pricing_plans (id, name, slug, price, currency, interval, status, tenant_id, created_at, updated_at)\n")
		sb.WriteString("VALUES\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000200', 'Free', 'free', 0, 'USD', 'monthly', 'active', '00000000-0000-0000-0000-000000000001', NOW(), NOW()),\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000201', 'Pro', 'pro', 29.00, 'USD', 'monthly', 'active', '00000000-0000-0000-0000-000000000001', NOW(), NOW()),\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000202', 'Enterprise', 'enterprise', 99.00, 'USD', 'monthly', 'active', '00000000-0000-0000-0000-000000000001', NOW(), NOW())\n")
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n\n")
	}

	if moduleSet["api_key_management"] {
		sb.WriteString("-- API Key Management: Default rate limit policies\n")
		sb.WriteString("INSERT INTO api_key_policies (id, name, requests_per_minute, requests_per_day, tenant_id, created_at, updated_at)\n")
		sb.WriteString("VALUES\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000300', 'default', 60, 10000, '00000000-0000-0000-0000-000000000001', NOW(), NOW()),\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000301', 'premium', 300, 100000, '00000000-0000-0000-0000-000000000001', NOW(), NOW())\n")
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n\n")
	}

	if moduleSet["notification_management"] {
		sb.WriteString("-- Notification Management: Default notification templates\n")
		sb.WriteString("INSERT INTO notification_templates (id, name, slug, channel, subject, body, tenant_id, created_at, updated_at)\n")
		sb.WriteString("VALUES\n")
		sb.WriteString("    ('00000000-0000-0000-0000-000000000400', 'Welcome Email', 'welcome-email', 'email', 'Welcome to {{.AppName}}', 'Hello {{.UserName}}, welcome!', '00000000-0000-0000-0000-000000000001', NOW(), NOW())\n")
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n\n")
	}

	sb.WriteString("COMMIT;\n")
	return sb.String()
}

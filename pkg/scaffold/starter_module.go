// Implements: REQ-002, REQ-016.
// Per: ADR-0017, ADR-0029.
// Discipline: C-14.

package scaffold

// starter_module.go owns GenerateStarterModule, the generator behind `pk new
// module`. It emits a starter-shaped PlatformKit module: a single Go package
// with a tenant-scoped store on the caller's shared *sql.DB and a REST
// handler that mounts behind the batteries-included starter's auth
// perimeter via starterapp.ModulePlugin (pk-apps/pkg/starterapp). This is
// deliberately a different shape than the multi-file, ports-and-catalog
// "business module" that module_templates_v2.go emits for the internal
// backend-kit composition model.

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
)

// starterModuleNamePattern is the canonical identifier shape for a starter
// module: a valid lowercase Go package name, snake_case, that also reads
// cleanly as a URL path segment and a SQL table name.
var starterModuleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// defaultStarterModuleDescription backstops StarterModuleOptions.Description
// when the caller leaves it blank.
const defaultStarterModuleDescription = "A custom PlatformKit module."

// StarterModuleOptions is the input contract for OSS starter-module
// generation.
type StarterModuleOptions struct {
	Name        string `json:"name"`        // snake_case package/module id
	Description string `json:"description"` // one-line module description
}

// GenerateStarterModule generates a starter-shaped module package: module.go
// (identity + composition), store.go (tenant-scoped persistence on the
// shared *sql.DB), handler.go (REST surface behind portslib.RequestActor),
// module_test.go (store tests against a real sqlite database), and a README
// carrying the starterapp.WithModules registration snippet.
func GenerateStarterModule(opts StarterModuleOptions) (ModuleResult, error) {
	if err := validateStarterModuleOptions(opts); err != nil {
		return ModuleResult{}, err
	}

	name := opts.Name
	description := strings.TrimSpace(opts.Description)
	if description == "" {
		description = defaultStarterModuleDescription
	}
	pascal := ToPascalCase(name)
	display := starterModuleDisplayName(name)

	dir := name + "/"
	snippet := renderStarterRegistrationSnippet(name)

	moduleGo, err := formatStarterGo(dir+"module.go", renderStarterModuleGo(name, pascal, display, description))
	if err != nil {
		return ModuleResult{}, err
	}
	storeGo, err := formatStarterGo(dir+"store.go", renderStarterStoreGo(name, pascal))
	if err != nil {
		return ModuleResult{}, err
	}
	handlerGo, err := formatStarterGo(dir+"handler.go", renderStarterHandlerGo(name, pascal))
	if err != nil {
		return ModuleResult{}, err
	}
	moduleTestGo, err := formatStarterGo(dir+"module_test.go", renderStarterModuleTestGo(name, pascal))
	if err != nil {
		return ModuleResult{}, err
	}

	files := []GeneratedFile{
		{Path: dir + "module.go", Content: moduleGo},
		{Path: dir + "store.go", Content: storeGo},
		{Path: dir + "handler.go", Content: handlerGo},
		{Path: dir + "module_test.go", Content: moduleTestGo},
		{Path: dir + "README.md", Content: renderStarterReadme(name, description, snippet)},
	}

	return ModuleResult{
		ModuleName: name,
		Files:      files,
		RegistrationCode: map[string]string{
			"snippet": snippet,
		},
	}, nil
}

func validateStarterModuleOptions(opts StarterModuleOptions) error {
	if !starterModuleNamePattern.MatchString(opts.Name) {
		return fmt.Errorf("starter module name %q must match ^[a-z][a-z0-9_]*$ (lowercase snake_case)", opts.Name)
	}
	return nil
}

// starterModuleDisplayName turns a snake_case module name into a spaced
// Pascal display name, e.g. "expense_notes" -> "Expense Notes".
func starterModuleDisplayName(name string) string {
	parts := strings.Split(name, "_")
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		words = append(words, strings.ToUpper(part[:1])+part[1:])
	}
	if len(words) == 0 {
		return name
	}
	return strings.Join(words, " ")
}

// formatStarterGo canonicalizes generated Go source with gofmt so emitted
// files read as hand-written regardless of the raw template's whitespace. A
// failure here means a template itself is broken, so it is reported as a
// generation error rather than emitted as broken output.
func formatStarterGo(path, src string) (string, error) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("format generated %s: %w", path, err)
	}
	return string(formatted), nil
}

// starterPluralize returns the naturally pluralized resource-name form used
// for the module's table name and REST route. name is returned unchanged
// when it already ends in "s" (e.g. "notes", "status" -> unchanged, no
// double pluralization), and "s" is appended otherwise (e.g. "note" ->
// "notes", "widget" -> "widgets").
func starterPluralize(name string) string {
	if strings.HasSuffix(name, "s") {
		return name
	}
	return name + "s"
}

// starterTokens replaces the template placeholders shared by every generated
// starter-module file. name is validated snake_case ([a-z][a-z0-9_]*), so it
// is always safe to splice directly into both Go identifiers/string literals
// and SQL/URL fragments; pascal is derived from it the same way. plural is
// the naturally pluralized resource form (starterPluralize) used for the
// table name and REST route so a name already ending in "s" is not
// double-pluralized. description is free text supplied by the caller, so it
// is pre-quoted with %q before substitution and the template must place
// __DESC_Q__ where a quoted Go string literal belongs (no surrounding quotes
// in the template itself).
func starterTokens(name, pascal, display, description string) *strings.Replacer {
	return strings.NewReplacer(
		"__NAME__", name,
		"__PASCAL__", pascal,
		"__DISPLAY__", display,
		"__DESC_Q__", fmt.Sprintf("%q", description),
		"__PLURAL__", starterPluralize(name),
	)
}

func renderStarterModuleGo(name, pascal, display, description string) string {
	tpl := `// Implements: REQ-016.
// Per: ADR-0017.
// Discipline: C-14.

package __NAME__

// module.go owns the __NAME__ module's singleton wiring: NewModule builds
// the store on the starter's shared *sql.DB, Compose returns the
// pkmodule.Composable the starter's catalog validates at compose time, and
// HTTPHandler exposes the wired REST handler for
// starterapp.ModulePlugin.RegisterRoutes.

import (
	"database/sql"

	pkmodule "github.com/septagon-oss/pk-core/pkg/module"
)

// Module metadata constants used by the starter catalog.
const (
	ModuleID          = "__NAME__"
	ModuleName        = "__DISPLAY__"
	ModuleDescription = __DESC_Q__
	ModuleVersion     = "0.1.0"
)

// Module is the __NAME__ starter module: a tenant-scoped store on the
// shared *sql.DB plus the REST handler that exposes it.
type Module struct {
	store   *Store
	handler *Handler
}

// NewModule builds the __NAME__ module on the starter's shared *sql.DB,
// creating its table if it does not already exist.
func NewModule(db *sql.DB) (*Module, error) {
	store, err := NewStore(db)
	if err != nil {
		return nil, err
	}
	return &Module{store: store, handler: NewHandler(store)}, nil
}

// Compose returns the module.Composable the starter's catalog consumes when
// validating port wiring. __PASCAL__ declares no cross-module dependencies,
// so it is a routes-only contribution (see starterapp.ModulePlugin.Compose).
func (m *Module) Compose() pkmodule.Composable {
	return pkmodule.Must(pkmodule.Metadata{
		ID:          ModuleID,
		Name:        ModuleName,
		Description: ModuleDescription,
		Version:     ModuleVersion,
	})
}

// HTTPHandler returns the wired HTTP handler so the host application can
// mount the module's routes via starterapp.ModulePlugin.RegisterRoutes.
func (m *Module) HTTPHandler() *Handler { return m.handler }
`
	return starterTokens(name, pascal, display, description).Replace(tpl)
}

func renderStarterStoreGo(name, pascal string) string {
	tpl := `// Implements: REQ-016.
// Per: ADR-0017.
// Discipline: C-14.

package __NAME__

// store.go owns the __NAME__ module's tenant-scoped persistence on the
// starter's shared *sql.DB. Every query filters by tenant_id so one tenant
// can never read or address another tenant's rows, and a cross-tenant
// lookup by id returns ErrNotFound rather than leaking existence.

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a __PASCAL__ row does not exist for the
// requesting tenant — including when the row exists but belongs to a
// different tenant.
var ErrNotFound = errors.New("__NAME__: not found")

// __PASCAL__ is a single tenant-scoped __NAME__ row.
type __PASCAL__ struct {
	ID        string ` + "`json:\"id\"`" + `
	TenantID  string ` + "`json:\"tenant_id\"`" + `
	OwnerID   string ` + "`json:\"owner_id\"`" + `
	Name      string ` + "`json:\"name\"`" + `
	CreatedAt string ` + "`json:\"created_at\"`" + `
}

// Store is the __NAME__ module's persistence boundary on the shared *sql.DB.
type Store struct {
	db *sql.DB
}

// NewStore opens the __NAME__ store on db, creating its table if it does not
// already exist.
func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(` + "`CREATE TABLE IF NOT EXISTS __PLURAL__ (\n\t\tid TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, owner_id TEXT NOT NULL,\n\t\tname TEXT NOT NULL, created_at TEXT NOT NULL)`" + `)
	if err != nil {
		return nil, fmt.Errorf("__NAME__: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Create inserts a new __NAME__ row. The caller (Handler) is responsible for
// assigning TenantID and OwnerID from the authenticated principal, never
// from client input.
func (s *Store) Create(item *__PASCAL__) error {
	_, err := s.db.Exec(
		` + "`INSERT INTO __PLURAL__ (id, tenant_id, owner_id, name, created_at) VALUES (?, ?, ?, ?, ?)`" + `,
		item.ID, item.TenantID, item.OwnerID, item.Name, item.CreatedAt,
	)
	return err
}

// List returns every __NAME__ row for tenantID, ordered by id.
func (s *Store) List(tenantID string) ([]*__PASCAL__, error) {
	rows, err := s.db.Query(
		` + "`SELECT id, tenant_id, owner_id, name, created_at FROM __PLURAL__ WHERE tenant_id = ? ORDER BY id`" + `,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*__PASCAL__{}
	for rows.Next() {
		item := &__PASCAL__{}
		if err := rows.Scan(&item.ID, &item.TenantID, &item.OwnerID, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Get returns the __NAME__ row with id for tenantID. It returns ErrNotFound
// both when the row does not exist and when it exists but belongs to a
// different tenant, so a cross-tenant lookup can never distinguish "missing"
// from "not yours".
func (s *Store) Get(tenantID, id string) (*__PASCAL__, error) {
	item := &__PASCAL__{}
	err := s.db.QueryRow(
		` + "`SELECT id, tenant_id, owner_id, name, created_at FROM __PLURAL__ WHERE id = ? AND tenant_id = ?`" + `,
		id, tenantID,
	).Scan(&item.ID, &item.TenantID, &item.OwnerID, &item.Name, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
`
	return starterTokens(name, pascal, "", "").Replace(tpl)
}

func renderStarterHandlerGo(name, pascal string) string {
	tpl := `// Implements: REQ-016.
// Per: ADR-0017.
// Discipline: C-14.

package __NAME__

// handler.go mounts the __NAME__ module's REST surface. It is registered via
// starterapp.ModulePlugin.RegisterRoutes, so it sits behind the starter's
// full security perimeter: identity resolution, the anonymous-mutation gate,
// and the request-body cap. Every handler binds tenant and owner from the
// authenticated principal via portslib.RequestActor — never from client
// input — so the server, not the caller, owns identity attribution.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/septagon-oss/pk-modules/pkg/portslib"
)

// Handler is the __NAME__ module's HTTP surface.
type Handler struct {
	store *Store
}

// NewHandler wires a Handler to store.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes mounts the __NAME__ routes for
// starterapp.ModulePlugin.RegisterRoutes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/v1/__PLURAL__", h)
	mux.Handle("/api/v1/__PLURAL__/", h)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, owner, ok := portslib.RequestActor(w, r)
	if !ok {
		return // 401 written by RequestActor
	}

	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/v1/__PLURAL__"), "/")
	switch {
	case id == "" && r.Method == http.MethodGet:
		items, err := h.store.List(tenant)
		writeJSON(w, http.StatusOK, items, err)
	case id == "" && r.Method == http.MethodPost:
		var item __PASCAL__
		if json.NewDecoder(r.Body).Decode(&item) != nil || strings.TrimSpace(item.Name) == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		item.TenantID, item.OwnerID = tenant, owner // server owns identity
		item.ID = strconv.FormatInt(time.Now().UnixNano(), 36)
		item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		writeJSON(w, http.StatusCreated, &item, h.store.Create(&item))
	case id != "" && r.Method == http.MethodGet:
		item, err := h.store.Get(tenant, id)
		writeJSON(w, http.StatusOK, item, err)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any, err error) {
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
`
	return starterTokens(name, pascal, "", "").Replace(tpl)
}

func renderStarterModuleTestGo(name, pascal string) string {
	tpl := `// Validates: REQ-016.
// Per: ADR-0017.
// Discipline: C-14.

package __NAME__

// module_test.go validates the __NAME__ store against a real sqlite
// database: create, list, and tenant-scoped get — including that a
// cross-tenant lookup by id returns ErrNotFound rather than another
// tenant's row.

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:__NAME___test.db?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func TestStoreCreateAndList(t *testing.T) {
	store := newTestStore(t)
	item := &__PASCAL__{ID: "1", TenantID: "tenant_a", OwnerID: "user_a", Name: "first", CreatedAt: "now"}
	if err := store.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, err := store.List("tenant_a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != "1" {
		t.Fatalf("List(tenant_a) = %+v, want one row with id 1", items)
	}
}

func TestStoreGet(t *testing.T) {
	store := newTestStore(t)
	item := &__PASCAL__{ID: "2", TenantID: "tenant_a", OwnerID: "user_a", Name: "second", CreatedAt: "now"}
	if err := store.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.Get("tenant_a", "2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "second" {
		t.Fatalf("Get() name = %q, want %q", got.Name, "second")
	}
}

func TestStoreGetCrossTenantMiss(t *testing.T) {
	store := newTestStore(t)
	item := &__PASCAL__{ID: "3", TenantID: "tenant_a", OwnerID: "user_a", Name: "third", CreatedAt: "now"}
	if err := store.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := store.Get("tenant_b", "3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(tenant_b, 3) error = %v, want ErrNotFound", err)
	}
}
`
	return starterTokens(name, pascal, "", "").Replace(tpl)
}

// renderStarterRegistrationSnippet renders the exact starterapp.WithModules
// wiring snippet a user pastes into their own main.go, mirroring the pattern
// documented on starterapp.WithModules and used by the widget example in
// pk-apps/examples/custommodule.
func renderStarterRegistrationSnippet(name string) string {
	tpl := "// In your main.go, add the module to the starter:\n" +
		"err := starterapp.Run(ctx, cfg, starterapp.WithModules(\n" +
		"\tfunc(env starterapp.ModuleEnv) (starterapp.ModulePlugin, error) {\n" +
		"\t\tm, err := __NAME__.NewModule(env.DB)\n" +
		"\t\tif err != nil {\n" +
		"\t\t\treturn starterapp.ModulePlugin{}, err\n" +
		"\t\t}\n" +
		"\t\treturn starterapp.ModulePlugin{\n" +
		"\t\t\tID:             __NAME__.ModuleID,\n" +
		"\t\t\tCompose:        m.Compose,\n" +
		"\t\t\tRegisterRoutes: m.HTTPHandler().RegisterRoutes,\n" +
		"\t\t}, nil\n" +
		"\t},\n" +
		"))\n"
	return strings.ReplaceAll(tpl, "__NAME__", name)
}

func renderStarterReadme(name, description, snippet string) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(name)
	b.WriteString("\n\n")
	b.WriteString(description)
	b.WriteString("\n\n")
	b.WriteString("Generated by `pk new module` as a starter-shaped PlatformKit module: a\n")
	b.WriteString("single Go package with a tenant-scoped store on the shared `*sql.DB` and a\n")
	b.WriteString("REST handler mounted behind the batteries-included starter's auth\n")
	b.WriteString("perimeter (see `starterapp.WithModules`).\n\n")
	b.WriteString("## Files\n\n")
	b.WriteString("- `module.go` — module identity, `NewModule`, `Compose`, `HTTPHandler`\n")
	b.WriteString("- `store.go` — tenant-scoped store on the shared `*sql.DB`\n")
	b.WriteString("- `handler.go` — REST handler behind `portslib.RequestActor`\n")
	b.WriteString("- `module_test.go` — store tests against a real sqlite database\n\n")
	b.WriteString("## Register it\n\n")
	b.WriteString("```go\n")
	b.WriteString(snippet)
	b.WriteString("```\n")
	return b.String()
}

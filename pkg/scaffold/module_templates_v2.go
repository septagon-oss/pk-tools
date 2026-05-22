// module_templates_v2.go — module scaffold templates that produce the
// canonical PlatformKit bootstrap (composer + runtime + manifest +
// entity-permissions + headers).
//
// Per: ADR-0017 (fx composition), ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"sort"
	"strings"
)

// filePurposeHeader emits the ADR-0029 leading comment block. It produces
// canonical lines like:
//
//	// foo.go — short description.
//	//
//	// Per: ADR-0017, ADR-0029.
//	// Discipline: C-14.
//
// Refs are classified by their prefix (REQ-, ADR-, C-) and grouped into
// the corresponding header line. A header always references at least one
// of REQ/ADR/C — the linter (check-file-purpose) enforces this.
func filePurposeHeader(filename, description string, refs ...string) string {
	var reqs, adrs, conventions []string
	for _, r := range refs {
		r = strings.TrimSpace(r)
		switch {
		case strings.HasPrefix(r, "REQ-"):
			reqs = append(reqs, r)
		case strings.HasPrefix(r, "ADR-"):
			adrs = append(adrs, r)
		case strings.HasPrefix(r, "C-"):
			conventions = append(conventions, r)
		}
	}
	if len(conventions) == 0 {
		// Every scaffolded file is part of the module discipline (C-14).
		conventions = []string{"C-14"}
	}
	if len(adrs) == 0 {
		// At minimum, the file follows ADR-0029 (which created this header).
		adrs = []string{"ADR-0029"}
	}

	var b strings.Builder
	b.WriteString("// ")
	b.WriteString(filename)
	b.WriteString(" — ")
	b.WriteString(description)
	b.WriteString(".\n")
	b.WriteString("//\n")
	if len(reqs) > 0 {
		b.WriteString("// Implements: ")
		b.WriteString(strings.Join(reqs, ", "))
		b.WriteString(".\n")
	}
	b.WriteString("// Per: ")
	b.WriteString(strings.Join(adrs, ", "))
	b.WriteString(".\n")
	b.WriteString("// Discipline: ")
	b.WriteString(strings.Join(conventions, ", "))
	b.WriteString(".\n")
	return b.String()
}

// generateModuleFiles produces the full canonical file set for a new
// PlatformKit business module.
func generateModuleFiles(opts ModuleOptions) []GeneratedFile {
	name := opts.Name
	description := opts.Description
	category := opts.Category
	archetype := opts.Archetype
	tier := opts.Tier
	domain := opts.Domain
	features := opts.Features
	tags := opts.Tags
	events := opts.Events
	ports := opts.Ports

	pascalName := ToPascalCase(name)
	displayName := moduleDisplayName(name)
	resourceName := moduleResourceName(name)
	resourceKebab := strings.ReplaceAll(resourceName, "_", "-")
	if resourceKebab == "" {
		resourceKebab = strings.ReplaceAll(name, "_", "-")
	}

	moduleTags := normalizeModuleTags(name, category, tags)
	moduleFeatures := normalizeFeatureNames(features)
	moduleEvents := normalizeEventNames(events)
	modulePorts := normalizePortNames(ports)

	files := []GeneratedFile{
		{Path: "module.go", Content: renderModuleGo(name, description, category, pascalName, moduleTags, archetype)},
		{Path: "metadata.go", Content: renderModuleMetadataGo(name, moduleFeatures)},
		{Path: "dependencies.go", Content: renderModuleDependenciesGo(name, modulePorts)},
		{Path: "events.go", Content: renderModuleEventsGo(name, moduleEvents)},
		{Path: "invocations.go", Content: renderModuleInvocationsGo(name, pascalName)},
		{Path: "admin.go", Content: renderModuleAdminGo(name, displayName, pascalName, resourceKebab)},
		{Path: "settings_provider.go", Content: renderModuleSettingsProviderGo(name, pascalName, displayName)},
		{Path: "authz.go", Content: renderModuleAuthzGo(name)},
		{Path: "entity_permissions.go", Content: renderModuleEntityPermissionsGo(name)},
		{Path: "README.md", Content: renderModuleReadme(name, description, category, archetype, moduleTags, moduleFeatures)},
		{Path: "module_smoke_test.go", Content: renderModuleSmokeTestGo(name)},
		{Path: "module.manifest.yaml", Content: renderModuleManifestYAML(name, description, category, resourceName, moduleTags, moduleFeatures, moduleEvents, modulePorts)},
		{Path: "contracts/module.go", Content: renderModuleContractsGo(name, description, category, moduleTags)},
		{Path: "contracts/providers.go", Content: renderModuleProvidersGo(name)},
		{Path: "contracts/permissions.go", Content: renderModulePermissionsGo(name, pascalName)},
		{Path: "contracts/routes.go", Content: renderModuleRoutesGo(name)},
		{Path: "contracts/provides/doc.go", Content: renderProvidesDocGo(name)},
	}

	if archetype != "infrastructure" {
		files = append(files, GeneratedFile{
			Path:    "features/README.md",
			Content: renderFeaturesReadme(name),
		})
	}

	if needsModuleMigrations(archetype) {
		files = append(files,
			GeneratedFile{Path: "migrations/README.md", Content: renderMigrationsReadme(name)},
			GeneratedFile{Path: "migrations/001_initial.up.sql", Content: renderInitialMigrationUp(name)},
			GeneratedFile{Path: "migrations/001_initial.down.sql", Content: renderInitialMigrationDown(name)},
		)
	}

	if opts.WithAssets {
		// Real placeholder files (not .gitkeep) so the //go:embed pattern
		// in assets_embed.go has at least one matching file in each
		// embedded directory. Operators replace these with real content.
		files = append(files,
			GeneratedFile{Path: "assets_embed.go", Content: renderAssetsEmbedGo(name)},
			GeneratedFile{Path: "assets_loader.go", Content: renderAssetsLoaderGo(name, pascalName)},
			GeneratedFile{Path: "translations/en.json", Content: renderPlaceholderTranslation(name, description)},
			GeneratedFile{Path: "design/tokens.json", Content: renderPlaceholderDesignTokens(name)},
			GeneratedFile{Path: "browser/js/" + resourceName + "_viewer.js", Content: renderPlaceholderBrowserJS(name, resourceName)},
		)
	}

	// Catalog hints: record what the operator must register in catalog/.
	// We surface this through registration code in ModuleResult, not as a
	// file in the new module directory, so we don't need to emit anything
	// here. Tier/domain are surfaced through module.manifest.yaml and
	// must be mirrored into module_contracts.yaml by the operator.
	_ = tier
	_ = domain

	for _, featureName := range moduleFeatures {
		featureFiles := generateFeatureFiles(name, featureName, nil)
		for _, file := range featureFiles {
			files = append(files, GeneratedFile{
				Path:    "features/" + featureName + "/" + file.Path,
				Content: file.Content,
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files
}

func normalizeModuleTags(name, category string, tags []string) []string {
	if len(tags) == 0 {
		tags = []string{category, moduleResourceName(name)}
	}

	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) == 0 {
		return []string{"business"}
	}
	return out
}

func normalizeFeatureNames(features []string) []string {
	seen := make(map[string]struct{}, len(features))
	out := make([]string, 0, len(features))
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		if _, ok := seen[feature]; ok {
			continue
		}
		seen[feature] = struct{}{}
		out = append(out, feature)
	}
	sort.Strings(out)
	return out
}

// normalizeEventNames trims and dedupes event names. Event names are
// dot-separated tokens like "spatial.asset.uploaded".
func normalizeEventNames(events []string) []string {
	seen := make(map[string]struct{}, len(events))
	out := make([]string, 0, len(events))
	for _, evt := range events {
		evt = strings.TrimSpace(evt)
		if evt == "" {
			continue
		}
		if _, ok := seen[evt]; ok {
			continue
		}
		seen[evt] = struct{}{}
		out = append(out, evt)
	}
	sort.Strings(out)
	return out
}

// normalizePortNames trims and dedupes port interface names. Names are
// short identifiers like "AuditService", "EventBus" — the templates
// emit them with the `ports.` qualifier.
func normalizePortNames(ports []string) []string {
	seen := make(map[string]struct{}, len(ports))
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func moduleDisplayName(name string) string {
	parts := strings.Split(name, "_")
	display := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		display = append(display, strings.ToUpper(part[:1])+part[1:])
	}
	return strings.Join(display, " ")
}

func moduleResourceName(name string) string {
	return strings.TrimSuffix(name, "_management")
}

func needsModuleMigrations(archetype string) bool {
	switch archetype {
	case "service", "specialized":
		return true
	default:
		return false
	}
}

func renderModuleGo(name, description, category, pascalName string, tags []string, archetype string) string {
	var imports []string
	imports = append(imports, `"example.com/platformkit/backend-kit/app/module"`)
	imports = append(imports, `"example.com/platformkit/backend-kit/app/module/providers/standard"`)
	if needsModuleMigrations(archetype) {
		imports = append([]string{`"embed"`}, imports...)
	}

	tagLiterals := quoteList(tags)

	migrationsBlock := ""
	if needsModuleMigrations(archetype) {
		migrationsBlock = `//go:embed migrations/*
var migrationsFS embed.FS

func init() {
	module.RegisterModuleMigrations(ModuleName, migrationsFS, "migrations")
}

`
	}

	header := filePurposeHeader("module.go", "module identity, singleton runtime, and migration embed", "ADR-0017")

	return fmt.Sprintf(`package %s

%s
import (
%s
)

const (
	ModuleName        = %q
	ModuleDescription = %q
	ModuleVersion     = "1.0.0"
	ModuleBasePath    = "/api/v1/%s"
	ModuleCategory    = %q
	ModuleAuthor      = "platformkit Team"
	ModuleLicense     = "MIT"
)

var ModuleTags = []string{%s}

// %sModule is the root runtime composition unit for this bounded context.
type %sModule struct {
	*standard.ModuleComposer
}

var _ module.Module = (*%sModule)(nil)

%s// moduleRuntime wraps the module singleton and exposes the standard
// catalog-shaped helpers (GetModule / NewModule / GetFeatures) as method
// values, removing the identical per-module wrapper boilerplate. The
// name avoids collision with the stdlib "runtime" package that some
// modules import.
var moduleRuntime = standard.NewRuntime(func() *%sModule {
	m := &%sModule{
		ModuleComposer: standard.NewComposer(
			moduleMetadata(),
			moduleComposerOptions()...,
		),
	}

	// Feed entity read-token gates into the "entity_permissions" fx group
	// consumed by admin_management.NewAggregatePermissionResolver. The
	// surface renderer fails closed for any registered row source that
	// lacks a matching gate (per ADR-0009).
	m.WithProviders(EntityReadPermissionsProvider())

	registerModuleInvocations(m)
	return m
})

// GetModule, NewModule, and GetFeatures are the catalog-facing helpers the
// application composition code references by name (e.g. %s.NewModule).
var (
	GetModule   = moduleRuntime.GetModule
	NewModule   = moduleRuntime.NewModule
	GetFeatures = moduleRuntime.GetFeatures
)
`, name, header, joinImports(imports), name, description, moduleResourceName(name), category, tagLiterals, pascalName, pascalName, pascalName, migrationsBlock, pascalName, pascalName, name)
}

func renderModuleMetadataGo(name string, features []string) string {
	imports := []string{
		`"example.com/platformkit/backend-kit/app/module"`,
		`"example.com/platformkit/backend-kit/app/module/providers/standard"`,
	}

	var featureImports []string
	var featureConstructors []string
	for _, feature := range features {
		alias := feature
		featureImports = append(featureImports, fmt.Sprintf(`%s "example.com/platformkit/business-modules/%s/features/%s"`, alias, name, feature))
		featureConstructors = append(featureConstructors, fmt.Sprintf("%s.New%sFeature()", alias, ToPascalCase(feature)))
	}
	sort.Strings(featureImports)

	if len(featureImports) > 0 {
		imports = append(imports, featureImports...)
	}

	featureBlock := ""
	if len(featureConstructors) > 0 {
		var b strings.Builder
		b.WriteString("\t\tstandard.WithFeatures(\n")
		for _, constructor := range featureConstructors {
			b.WriteString("\t\t\t")
			b.WriteString(constructor)
			b.WriteString(",\n")
		}
		b.WriteString("\t\t),\n")
		featureBlock = b.String()
	}

	header := filePurposeHeader("metadata.go", "module metadata projection and composer options", "ADR-0017")

	return fmt.Sprintf(`package %s

%s
import (
%s
)

func moduleMetadata() module.ModuleMetadata {
	return module.ModuleMetadata{
		Name:        ModuleName,
		Description: ModuleDescription,
		Version:     ModuleVersion,
		BasePath:    ModuleBasePath,
		Category:    ModuleCategory,
		Author:      ModuleAuthor,
		License:     ModuleLicense,
		Tags:        append([]string(nil), ModuleTags...),
	}
}

func moduleComposerOptions() []standard.ComposerOption {
	options := []standard.ComposerOption{
%s	}
	options = append(options, moduleDependencyOptions()...)
	options = append(options, moduleEventOptions()...)
	return options
}
`, name, header, joinImports(imports), featureBlock)
}

func renderModuleDependenciesGo(name string, extraPorts []string) string {
	header := filePurposeHeader("dependencies.go", "cross-module dependency declarations via standard.WithDep + module.RequiresPort/OptionalPort", "ADR-0009", "ADR-0017")

	// Standard registrars every module needs.
	defaultDeps := []string{
		fmt.Sprintf(`		standard.WithDep(module.RequiresPort[ports.AdminRegistrar](module.PortSpec{Purpose: "Register %s surfaces in admin registry", Category: module.DependencyCategoryUI, SubCategory: "admin", PreferredProvider: "admin_management"})),`, moduleResourceName(name)),
		fmt.Sprintf(`		standard.WithDep(module.RequiresPort[ports.HealthRegistrar](module.PortSpec{Purpose: "Register %s health checks", Category: module.DependencyCategoryMonitoring, SubCategory: "health", PreferredProvider: "health_management"})),`, moduleResourceName(name)),
		fmt.Sprintf(`		standard.WithDep(module.RequiresPort[ports.TranslationRegistrar](module.PortSpec{Purpose: "Register %s translations", Category: module.DependencyCategoryInfrastructure, SubCategory: "i18n", PreferredProvider: "translation_management"})),`, moduleResourceName(name)),
		fmt.Sprintf(`		standard.WithDep(module.OptionalPort[ports.SettingsRegistrar](module.PortSpec{Purpose: "Register %s defaults and configuration", Category: module.DependencyCategoryUI, SubCategory: "settings", PreferredProvider: "admin_management"})),`, moduleResourceName(name)),
	}

	// Extra ports the operator declared on the command line.
	var extraDepLines []string
	for _, p := range extraPorts {
		extraDepLines = append(extraDepLines, fmt.Sprintf(`		standard.WithDep(module.OptionalPort[ports.%s](module.PortSpec{Purpose: "TODO: describe why %s needs ports.%s", Category: module.DependencyCategoryData, SubCategory: %q, PreferredProvider: "TODO_provider_module"})),`, p, name, p, strings.ToLower(p)))
	}

	var b strings.Builder
	for _, line := range defaultDeps {
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, line := range extraDepLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/backend-kit/app/module"
	"example.com/platformkit/backend-kit/app/module/providers/standard"
	"example.com/platformkit/business-modules/ports"
)

func moduleDependencyOptions() []standard.ComposerOption {
	return []standard.ComposerOption{
%s	}
}
`, name, header, b.String())
}

func renderModuleEventsGo(name string, events []string) string {
	header := filePurposeHeader("events.go", "declared event contracts emitted by this module", "ADR-0018")

	if len(events) == 0 {
		// No declared events — emit a function returning nil so it
		// composes cleanly into moduleComposerOptions. Operator can
		// add events by editing this file or rerunning scaffold.
		return fmt.Sprintf(`package %s

%s
import "example.com/platformkit/backend-kit/app/module/providers/standard"

// moduleEventOptions returns the standard.WithEvent declarations for every
// event this module emits. Add new events via standard.WithEvent and emit
// them through the outbox (ADR-0007).
func moduleEventOptions() []standard.ComposerOption {
	return nil
}
`, name, header)
	}

	var b strings.Builder
	for _, evt := range events {
		b.WriteString(fmt.Sprintf(`		standard.WithEvent(%q, "TODO: describe %s", map[string]any{
			"tenantId":  "string",
			"timestamp": "timestamp",
		}),`, evt, evt))
		b.WriteString("\n")
	}

	return fmt.Sprintf(`package %s

%s
import "example.com/platformkit/backend-kit/app/module/providers/standard"

// moduleEventOptions returns the standard.WithEvent declarations for every
// event this module emits. The payload schemas below are starter stubs —
// fill in the real shape before emitting any event from production code.
func moduleEventOptions() []standard.ComposerOption {
	return []standard.ComposerOption{
%s	}
}
`, name, header, b.String())
}

func renderModuleInvocationsGo(name, pascalName string) string {
	header := filePurposeHeader("invocations.go", "fx invocation registrations for admin, health, translations, and settings", "ADR-0017")

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/business-modules/internal/moduleproviders"
	"example.com/platformkit/business-modules/ports"
	"go.uber.org/fx"
)

type moduleOptionalRegistrars struct {
	fx.In

	SettingsRegistrar ports.SettingsRegistrar `+"`"+`optional:"true"`+"`"+`
}

func registerModuleInvocations(m *%sModule) {
	m.ModuleComposer.WithInvocations(func(adminRegistrar ports.AdminRegistrar) error {
		return adminRegistrar.RegisterProvider(ModuleName, New%sAdmin())
	})

	m.ModuleComposer.WithInvocations(func(healthReg ports.HealthRegistrar) error {
		return healthReg.RegisterProvider(ModuleName, moduleproviders.NewBasicHealthProvider(
			ModuleName,
			ModuleName,
			ModuleDescription,
		))
	})

	m.ModuleComposer.WithInvocations(func(translationReg ports.TranslationRegistrar) error {
		return translationReg.RegisterProvider(ModuleName, moduleproviders.NewModuleMetadataTranslationProvider(
			ModuleName,
			ModuleDescription,
		))
	})

	m.ModuleComposer.WithInvocations(func(regs moduleOptionalRegistrars) error {
		if regs.SettingsRegistrar == nil {
			return nil
		}
		return regs.SettingsRegistrar.RegisterSettingsProvider(ModuleName, New%sSettingsProvider())
	})
}
`, name, header, pascalName, pascalName, pascalName)
}

func renderModuleAdminGo(name, displayName, pascalName, resourceKebab string) string {
	header := filePurposeHeader("admin.go", "admin capability provider exposing sidebar entries and read/write permissions", "ADR-0009")

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/backend-kit/app/module"
	"example.com/platformkit/backend-kit/app/module/helpers"
	contracts "example.com/platformkit/business-modules/%s/contracts"
)

func New%sAdmin() *helpers.ModuleAdmin {
	return helpers.NewModuleAdmin(ModuleName).
		DisplayName(%q).
		Description(ModuleDescription).
		Icon("box").
		Category(module.AdminCategoryContent).
		Pages(helpers.AdminPage{
			ID:          ModuleName,
			Title:       %q,
			Description: ModuleDescription,
			Route:       "/admin/%s",
			Order:       100,
			Permissions: []string{contracts.Permission%sView},
		}).
		Capabilities(
			module.AdminCapability{
				ID:          contracts.Permission%sView,
				Name:        "View %s",
				Description: "View %s data",
				Category:    %q,
				Resource:    %q,
				Actions:     []string{"read", "list"},
			},
			module.AdminCapability{
				ID:          contracts.Permission%sManage,
				Name:        "Manage %s",
				Description: "Create, update, and delete %s data",
				Category:    %q,
				Resource:    %q,
				Actions:     []string{"create", "update", "delete"},
			},
		).
		Build()
}
`, name, header,
		name,
		pascalName,
		displayName,
		displayName,
		resourceKebab,
		pascalName,
		pascalName,
		displayName,
		displayName,
		moduleResourceName(name),
		moduleResourceName(name),
		pascalName,
		displayName,
		displayName,
		moduleResourceName(name),
		moduleResourceName(name),
	)
}

func renderModuleSettingsProviderGo(name, pascalName, displayName string) string {
	header := filePurposeHeader("settings_provider.go", "tenant-scoped settings exposed by this module", "ADR-0009")

	return fmt.Sprintf(`package %s

%s
import "example.com/platformkit/business-modules/ports"

type %sSettingsProvider struct{}

func New%sSettingsProvider() ports.SettingsProvider {
	return &%sSettingsProvider{}
}

func (p *%sSettingsProvider) GetSettings() []ports.SettingDefinition {
	return []ports.SettingDefinition{
		{
			ID:          ModuleName + ".enabled",
			Key:         "module.enabled",
			Name:        %q + " Enabled",
			Description: "Enable or disable module-level runtime behavior for " + ModuleName + ".",
			Type:        "bool",
			DefaultValue: true,
			Category:    "general",
			Order:       10,
		},
	}
}
`, name, header, pascalName, pascalName, pascalName, pascalName, displayName)
}

func renderModuleAuthzGo(name string) string {
	header := filePurposeHeader("authz.go", "canonical permission tokens exported by this module", "ADR-0009")

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/backend-kit/security/authz"
	contracts "example.com/platformkit/business-modules/%s/contracts"
)

// ModulePermissionTokens declares the canonical permission tokens exported by this module.
var ModulePermissionTokens = authz.MustNormalizePermissionTokens(contracts.ModulePermissions())
`, name, header, name)
}

func renderModuleEntityPermissionsGo(name string) string {
	header := filePurposeHeader("entity_permissions.go", "read-token gates for entities surfaced by this module", "ADR-0009")

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/business-modules/ports"
	"go.uber.org/fx"
)

// entityReadPermissions declares the read-token gate that admin_management's
// AggregatePermissionResolver consults before rendering an entity surface.
// Add an entry to ByEntity for every entity this module exposes — the
// surface renderer fails closed when a registered row source lacks a
// matching gate.
func entityReadPermissions() ports.EntityReadPermissions {
	return ports.EntityReadPermissions{
		ModuleID: ModuleName,
		ByEntity: map[string][]string{
			// "EntityName": {"%s:read"},
		},
	}
}

// EntityReadPermissionsProvider feeds entityReadPermissions into the
// "entity_permissions" fx group consumed by
// admin_management.NewAggregatePermissionResolver.
func EntityReadPermissionsProvider() any {
	return fx.Annotate(
		entityReadPermissions,
		fx.ResultTags(`+"`"+`group:"entity_permissions"`+"`"+`),
	)
}
`, name, header, moduleResourceName(name))
}

func renderModuleReadme(name, description, category, archetype string, tags, features []string) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(name)
	b.WriteString("\n\n")
	b.WriteString(description)
	b.WriteString("\n\n")
	b.WriteString("## Metadata\n\n")
	b.WriteString("- Archetype: `")
	b.WriteString(archetype)
	b.WriteString("`\n")
	b.WriteString("- Category: `")
	b.WriteString(category)
	b.WriteString("`\n")
	b.WriteString("- Base path: `/api/v1/")
	b.WriteString(moduleResourceName(name))
	b.WriteString("`\n")
	if len(tags) > 0 {
		b.WriteString("- Tags: `")
		b.WriteString(strings.Join(tags, ", "))
		b.WriteString("`\n")
	}
	b.WriteString("\n## Responsibilities\n\n")
	b.WriteString("- Owns the module-local contracts, providers, and feature composition for this bounded context.\n")
	b.WriteString("- Exposes cross-module capabilities through `contracts/` and `contracts/provides/`.\n")
	if needsModuleMigrations(archetype) {
		b.WriteString("- Carries its own Goose-compatible migrations under `migrations/`.\n")
	}
	b.WriteString("\n## Features\n\n")
	if len(features) == 0 {
		b.WriteString("- _Add feature packages under `features/` as this module grows._\n")
	} else {
		for _, feature := range features {
			b.WriteString("- `")
			b.WriteString(feature)
			b.WriteString("`\n")
		}
	}
	b.WriteString("\n## Structure\n\n")
	b.WriteString("- `module.go`: module identity and runtime entrypoints\n")
	b.WriteString("- `metadata.go`: module metadata projection and composer options\n")
	b.WriteString("- `dependencies.go`: cross-module dependency declarations\n")
	b.WriteString("- `invocations.go`: registrar wiring for admin, health, translations, and settings\n")
	b.WriteString("- `entity_permissions.go`: read-token gates for surfaced entities\n")
	b.WriteString("- `module.manifest.yaml`: declared contract for the module catalog\n")
	b.WriteString("- `contracts/`: exported module contracts and generated roots\n")
	b.WriteString("- `features/`: internal feature decomposition\n")
	if needsModuleMigrations(archetype) {
		b.WriteString("- `migrations/`: append-only database migrations\n")
	}
	b.WriteString("\n## Notes\n\n")
	b.WriteString("- Cross-module communication must stay behind `platformkit-business-modules/ports` interfaces (ADR-0009).\n")
	b.WriteString("- Events go through the outbox, not direct event-bus publishes (ADR-0007).\n")
	b.WriteString("- Every emitted event must be declared via `standard.WithEvent` (ADR-0018).\n")
	b.WriteString("- Every public port method must work over both HTTP and NATS eventbus (ADR-0019).\n")
	b.WriteString("- After adding or changing features, rerun module normalization and verification.\n")
	return b.String()
}

func renderModuleSmokeTestGo(name string) string {
	header := filePurposeHeader("module_smoke_test.go", "module contract assertion", "ADR-0029")

	return fmt.Sprintf(`package %s

%s
import (
	"testing"

	"example.com/platformkit/business-modules/testutil"
)

func TestModuleContract(t *testing.T) {
	testutil.AssertModuleContract(t, GetModule(), ModuleName)
}
`, name, header)
}

func renderModuleContractsGo(name, description, category string, tags []string) string {
	header := filePurposeHeader("module.go", "metadata constants mirrored for cross-module consumers", "ADR-0009")

	return fmt.Sprintf(`package contracts

%s
const (
	ModuleName        = %q
	ModuleDescription = %q
	ModuleVersion     = "1.0.0"
	ModuleBasePath    = "/api/v1/%s"
	ModuleCategory    = %q
	ModuleAuthor      = "platformkit Team"
	ModuleLicense     = "MIT"
)

var ModuleTags = []string{%s}
`, header, name, description, moduleResourceName(name), category, quoteList(tags))
}

func renderModuleProvidersGo(name string) string {
	header := filePurposeHeader("providers.go", "type aliases re-exporting provider-side contracts", "ADR-0009")

	return fmt.Sprintf(`package contracts

%s
// Provider-side aliases belong here once %s exposes stable contracts from
// contracts/provides/.
//
// Example:
// type Service = provides.%sService
`, header, name, ToPascalCase(name))
}

func renderModulePermissionsGo(name, pascalName string) string {
	header := filePurposeHeader("permissions.go", "permission token constants exposed by this module", "ADR-0009")

	return fmt.Sprintf(`package contracts

%s
const (
	Permission%sView   = %q
	Permission%sManage = %q
)

func ModulePermissions() []string {
	return []string{
		Permission%sManage,
		Permission%sView,
	}
}
`, header, pascalName, name+":view", pascalName, name+":manage", pascalName, pascalName)
}

func renderModuleRoutesGo(name string) string {
	header := filePurposeHeader("routes.go", "route-level constants exposed for cross-module reference", "ADR-0009")

	return fmt.Sprintf(`package contracts

%s
import "example.com/platformkit/backend-kit/app/module"

const (
	ModuleAPIBasePath = ModuleBasePath
)

func ModuleEndpoints() []module.EndpointDefinition {
	return []module.EndpointDefinition{}
}
`, header)
}

func renderProvidesDocGo(name string) string {
	header := filePurposeHeader("doc.go", "package docstring for the provides contract surface", "ADR-0009")

	return fmt.Sprintf(`// Package provides contains provider-side interfaces and DTOs exported by
// %s.
package provides

%s`, name, header)
}

func renderModuleManifestYAML(name, description, category, resourceName string, tags, features, events, ports []string) string {
	var b strings.Builder
	b.WriteString("apiVersion: platformkit.dev/v1\n")
	b.WriteString("kind: ModuleManifest\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	b.WriteString("  version: 0.1.0\n")
	b.WriteString("  description: " + yamlString(description) + "\n")
	b.WriteString("  author: platformkit Team\n")
	b.WriteString("  license: MIT\n")
	if len(tags) > 0 {
		b.WriteString("  tags:\n")
		for _, t := range tags {
			b.WriteString("    - " + t + "\n")
		}
	}
	b.WriteString("spec:\n")
	b.WriteString("  basePath: /api/v1/" + resourceName + "\n")
	b.WriteString("  category: " + category + "\n")
	b.WriteString("  composition:\n")
	b.WriteString("    supportedModes:\n")
	b.WriteString("      - local\n")
	b.WriteString("    stateOwner: true\n")
	b.WriteString("    requiredDepsFailFast: false\n")
	b.WriteString("  dependencies:\n")
	b.WriteString("    required:\n")
	b.WriteString("      - interface: ports.AdminRegistrar\n")
	b.WriteString("        description: Register " + resourceName + " surfaces in admin registry\n")
	b.WriteString("        version: \"*\"\n")
	b.WriteString("      - interface: ports.HealthRegistrar\n")
	b.WriteString("        description: Register " + resourceName + " health checks\n")
	b.WriteString("        version: \"*\"\n")
	b.WriteString("      - interface: ports.TranslationRegistrar\n")
	b.WriteString("        description: Register " + resourceName + " translations\n")
	b.WriteString("        version: \"*\"\n")
	b.WriteString("    optional:\n")
	b.WriteString("      - interface: ports.SettingsRegistrar\n")
	b.WriteString("        description: Register " + resourceName + " defaults and configuration\n")
	b.WriteString("        version: \"*\"\n")
	for _, p := range ports {
		b.WriteString("      - interface: ports." + p + "\n")
		b.WriteString("        description: TODO describe why " + name + " needs ports." + p + "\n")
		b.WriteString("        version: \"*\"\n")
	}
	if len(features) > 0 {
		b.WriteString("  features:\n")
		for _, f := range features {
			b.WriteString("    - id: " + f + "\n")
			b.WriteString("      name: " + moduleDisplayName(f) + "\n")
			b.WriteString("      description: TODO describe the " + f + " feature\n")
			b.WriteString("      version: 0.1.0\n")
			b.WriteString("      category: " + category + "\n")
			b.WriteString("      enabled: true\n")
		}
	} else {
		b.WriteString("  features: []\n")
	}
	if len(events) > 0 {
		b.WriteString("  events:\n")
		for _, e := range events {
			b.WriteString("    - name: " + e + "\n")
			b.WriteString("      description: TODO describe " + e + "\n")
		}
	} else {
		b.WriteString("  events: []\n")
	}
	b.WriteString("  permissions:\n")
	b.WriteString("    permissions:\n")
	b.WriteString("      - token: " + name + ":view\n")
	b.WriteString("        description: Allow viewing " + resourceName + " resources.\n")
	b.WriteString("      - token: " + name + ":manage\n")
	b.WriteString("        description: Allow managing " + resourceName + " resources.\n")
	b.WriteString("  integrations:\n")
	b.WriteString("    authz:\n")
	b.WriteString("      protectedPrefixes:\n")
	b.WriteString("        - /api/v1/" + resourceName + "\n")
	b.WriteString("      permissionTokenFormat: resource:action\n")
	if len(events) > 0 {
		b.WriteString("    events:\n")
		b.WriteString("      emitted:\n")
		for _, e := range events {
			b.WriteString("        - " + e + "\n")
		}
	}
	return b.String()
}

// yamlString wraps strings that contain YAML-significant characters in
// double quotes. Plain alphanumeric + space + simple punctuation stays
// bare for readability.
func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if r == ':' || r == '#' || r == '&' || r == '*' || r == '!' || r == '|' || r == '>' || r == '%' || r == '@' || r == '`' {
			return fmt.Sprintf("%q", s)
		}
	}
	return s
}

func renderInitialMigrationUp(name string) string {
	return fmt.Sprintf(`-- +goose Up
-- 001_initial.up.sql — initial schema for %s.
--
-- Replace this stub with the real CREATE TABLE statements when you add
-- entities. Every table this module owns must include tenant_id and the
-- standard audit columns (created_at, updated_at, deleted_at). Migrations
-- are append-only: never edit a committed migration; add a new one with
-- a higher sequence number.
`, name)
}

func renderInitialMigrationDown(name string) string {
	return fmt.Sprintf(`-- +goose Down
-- 001_initial.down.sql — inverse of 001_initial.up.sql for %s.
--
-- Mirror the destructive inverse of the up migration here.
`, name)
}

func renderAssetsEmbedGo(name string) string {
	header := filePurposeHeader("assets_embed.go", "browser asset embed for translations, design tokens, and JS controllers", "ADR-0024")

	return fmt.Sprintf(`package %s

%s
import (
	"embed"

	"example.com/platformkit/business-modules/internal/moduleassets"
)

//go:embed translations/*.json design/tokens.json browser/js/*.js
var moduleAssetFS embed.FS

func init() {
	moduleassets.MustRegisterBrowserAssets(ModuleName, moduleAssetFS)
}
`, name, header)
}

func renderAssetsLoaderGo(name, pascalName string) string {
	header := filePurposeHeader("assets_loader.go", "loader exposing embedded translations and design tokens to the platform", "ADR-0024")

	return fmt.Sprintf(`package %s

%s
import "example.com/platformkit/business-modules/internal/moduleassets"

// AssetsLoader exposes translations and design tokens embedded in this
// module to the platform's translation and design-token registrars.
type AssetsLoader struct {
	*moduleassets.EmbeddedLoader
}

func NewAssetsLoader() *AssetsLoader {
	return &AssetsLoader{
		EmbeddedLoader: moduleassets.NewEmbeddedLoader(moduleAssetFS),
	}
}

func (l *AssetsLoader) LoadTranslation(locale string) (map[string]any, error) {
	return l.LoadTranslationTree(locale)
}

func (l *AssetsLoader) LoadDesignTokens() ([]byte, error) {
	return l.LoadDesignTokensJSON()
}

// Compile-time guard so the loader keeps satisfying its expected shape.
var _ interface {
	LoadTranslation(string) (map[string]any, error)
	LoadDesignTokens() ([]byte, error)
} = (*AssetsLoader)(nil)
`, name, header)
}

func renderPlaceholderTranslation(name, description string) string {
	return fmt.Sprintf(`{
  "module": {
    "name": %q,
    "description": %q
  }
}
`, moduleDisplayName(name), description)
}

func renderPlaceholderDesignTokens(name string) string {
	return fmt.Sprintf(`{
  "$schema": "https://platformkit.dev/schemas/design-tokens.json",
  "module": %q,
  "tokens": {}
}
`, name)
}

func renderPlaceholderBrowserJS(name, resourceName string) string {
	return fmt.Sprintf(`// %s_viewer.js — placeholder browser controller for the %s module.
//
// Replace this stub with the real controller once you ship a feature that
// owns browser behavior. The //go:embed pattern in assets_embed.go
// requires at least one .js file under browser/js, so this stub exists
// purely to make the module compile.
(function () {
  if (typeof window === "undefined") {
    return;
  }
  window.%sViewer = window.%sViewer || {};
})();
`, resourceName, name, ToPascalCase(resourceName), ToPascalCase(resourceName))
}

func renderFeaturesReadme(name string) string {
	return fmt.Sprintf(`# %s features

Add one subdirectory per feature. Each feature should at minimum own:

- `+"`feature.go`"+`
- `+"`handler.go`"+`
- `+"`service.go`"+`
- `+"`e2e.go`"+`

Wire new feature constructors into `+"`metadata.go`"+` via `+"`standard.WithFeatures(...)`"+`.
`, name)
}

func renderMigrationsReadme(name string) string {
	return fmt.Sprintf(`# %s migrations

Store append-only Goose migrations here using `+"`NNN_name.up.sql`"+` and
`+"`NNN_name.down.sql`"+` pairs. The initial stub lives in `+"`001_initial.up.sql`"+`
and `+"`001_initial.down.sql`"+` — replace it with real CREATE TABLE statements
when you add entities. Never edit committed migrations; add new ones
with higher sequence numbers.
`, name)
}

func joinImports(imports []string) string {
	sort.Strings(imports)
	var b strings.Builder
	for _, path := range imports {
		b.WriteString("\t")
		b.WriteString(path)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func quoteList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

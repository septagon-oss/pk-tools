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
		{Path: "surfaces.go", Content: renderModuleSurfacesGo(name, displayName, pascalName, resourceKebab)},
		{Path: "settings_provider.go", Content: renderModuleSettingsProviderGo(name)},
		{Path: "authz.go", Content: renderModuleAuthzGo(name)},
		{Path: "entity_permissions.go", Content: renderModuleEntityPermissionsGo(name)},
		{Path: "README.md", Content: renderModuleReadme(name, description, category, archetype, moduleTags, moduleFeatures)},
		{Path: "module_smoke_test.go", Content: renderModuleSmokeTestGo(name)},
		{Path: "module.manifest.yaml", Content: renderModuleManifestYAML(name, description, category, resourceName, moduleTags, moduleFeatures, moduleEvents, modulePorts)},
		{Path: "module.skills.yaml", Content: renderModuleSkillsYAML(name, description, moduleTags)},
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
	imports = append(imports, `"example.com/platformkit/business-modules/ports"`)
	imports = append(imports, `"go.uber.org/fx"`)
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

	// Modern surface contribution (replaces legacy AdminRegistrar registration;
	// ADR-0001 registrar finale). The fx-group value feeds the admin collector /
	// validators; the name-tagged typed provider supports interface discovery
	// without a duplicate-provide error across registrar-less modules.
	m.WithProviders(
		fx.Annotate(
			func() ports.ModuleSurfaceContribution { return moduleSurfaceContribution() },
			fx.ResultTags(`+"`"+`group:"module_surface_contributions"`+"`"+`),
		),
		fx.Annotate(
			New%sSurfaceContributionProvider,
			fx.As(new(ports.ModuleSurfaceContributionProvider)),
			fx.ResultTags(`+"`"+`name:"%s_surface_contribution_provider"`+"`"+`),
		),
	)

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
`, name, header, joinImports(imports), name, description, moduleResourceName(name), category, tagLiterals, pascalName, pascalName, pascalName, migrationsBlock, pascalName, pascalName, pascalName, name, name)
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
	header := filePurposeHeader("dependencies.go", "cross-module dependency declarations via standard.WithDep + module.RequiresPort/OptionalPort", "ADR-0001", "ADR-0009", "ADR-0017")

	// Modern wiring (ADR-0001 registrar finale): the admin surface is contributed
	// declaratively via ModuleSurfaceContribution (surfaces.go / module.go) and
	// health via the "health_providers" fx group (invocations.go) — no
	// Admin/Health/Settings registrar dependencies. Translation registration is
	// still a typed port dependency on translation_management's provides surface.
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`		standard.WithDep(module.RequiresPort[translationprovides.TranslationRegistrar](module.PortSpec{Purpose: "Register %s translations", Category: module.DependencyCategoryInfrastructure, SubCategory: "i18n", PreferredProvider: "translation_management"})),`, moduleResourceName(name)))
	b.WriteString("\n")

	// Extra ports the operator declared on the command line.
	for _, p := range extraPorts {
		b.WriteString(fmt.Sprintf(`		standard.WithDep(module.OptionalPort[ports.%s](module.PortSpec{Purpose: "TODO: describe why %s needs ports.%s", Category: module.DependencyCategoryData, SubCategory: %q, PreferredProvider: "TODO_provider_module"})),`, p, name, p, strings.ToLower(p)))
		b.WriteString("\n")
	}

	// The ports package is only imported when the operator declared extra ports;
	// otherwise importing it would be an unused-import compile error.
	portsImport := ""
	if len(extraPorts) > 0 {
		portsImport = "\n\t\"example.com/platformkit/business-modules/ports\""
	}

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/backend-kit/app/module"
	"example.com/platformkit/backend-kit/app/module/providers/standard"%s
	translationprovides "example.com/platformkit/business-modules/translation_management/contracts/provides"
)

func moduleDependencyOptions() []standard.ComposerOption {
	return []standard.ComposerOption{
%s	}
}
`, name, header, portsImport, b.String())
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
	header := filePurposeHeader("invocations.go", "fx invocation registrations for health and translations", "ADR-0001", "ADR-0017")

	return fmt.Sprintf(`package %s

%s
import (
	healthprovides "example.com/platformkit/business-modules/health_management/contracts/provides"
	"example.com/platformkit/business-modules/internal/moduleproviders"
	translationprovides "example.com/platformkit/business-modules/translation_management/contracts/provides"
	"go.uber.org/fx"
)

func registerModuleInvocations(m *%sModule) {
	// Modern wiring (ADR-0001 registrar finale): the admin surface is contributed
	// via ModuleSurfaceContribution (surfaces.go / module.go), health via the
	// "health_providers" fx group, and translations via the translation
	// registrar port — no legacy Admin/Settings registrar invocations.

	// Health provider contributed into the "health_providers" fx group; the
	// health_management module collects and registers it.
	m.ModuleComposer.WithProviders(fx.Annotate(
		func() healthprovides.HealthContribution {
			return healthprovides.HealthContribution{
				ModuleID: ModuleName,
				Provider: moduleproviders.NewBasicHealthProvider(
					ModuleName,
					ModuleName,
					ModuleDescription,
				),
			}
		},
		fx.ResultTags(`+"`"+`group:"health_providers"`+"`"+`),
	))

	m.ModuleComposer.WithInvocations(func(translationReg translationprovides.TranslationRegistrar) error {
		return translationReg.RegisterProvider(ModuleName, moduleproviders.NewModuleMetadataTranslationProvider(
			ModuleName,
			ModuleDescription,
		))
	})
}
`, name, header, pascalName)
}

func renderModuleSurfacesGo(name, displayName, pascalName, resourceKebab string) string {
	header := filePurposeHeader("surfaces.go", "declarative admin surface contribution (replaces the legacy AdminRegistrar registration path)", "ADR-0001", "ADR-0009", "ADR-0017")

	return fmt.Sprintf(`package %s

%s
import (
	contracts "example.com/platformkit/business-modules/%s/contracts"
	"example.com/platformkit/business-modules/ports"
)

// moduleSurfaceContribution registers this module's admin page. The route ID
// equals the module name (the un-prefixed legacy AdminPage ID convention) so
// section renderers dispatching on that ID keep matching; PagePattern stays
// Unknown because the scaffold ships no custom section renderer — the default
// entity-table renderer handles any RegisterEntity surfaces.
func moduleSurfaceContribution() ports.ModuleSurfaceContribution {
	return ports.CanonicalizeAdminSurfaceContribution(ports.ModuleSurfaceContribution{
		ModuleID: ModuleName,
		Routes: []ports.ModuleSurfaceRoute{
			{
				ID:             ModuleName,
				Path:           "/admin/%s",
				Title:          ports.SurfaceText{Fallback: %q},
				NavLabel:       ports.SurfaceText{Fallback: %q},
				Icon:           "box",
				Order:          100,
				PagePattern:    ports.PagePatternHintUnknown,
				CapabilityTags: []string{contracts.Permission%sView},
				Targets:        []ports.SurfaceTarget{ports.SurfaceTargetAdmin},
			},
		},
	})
}

// %sSurfaceContributionProvider is the modern typed surface provider (replaces
// the legacy AdminRegistrar.RegisterProvider invocation).
type %sSurfaceContributionProvider struct {
	contribution ports.ModuleSurfaceContribution
}

func New%sSurfaceContributionProvider() *%sSurfaceContributionProvider {
	return &%sSurfaceContributionProvider{contribution: moduleSurfaceContribution()}
}

func (p *%sSurfaceContributionProvider) GetSurfaceContribution() ports.ModuleSurfaceContribution {
	return p.contribution
}

// Compile-time guard.
var _ ports.ModuleSurfaceContributionProvider = (*%sSurfaceContributionProvider)(nil)
`, name, header,
		name,
		resourceKebab,
		displayName,
		displayName,
		pascalName,
		pascalName,
		pascalName,
		pascalName,
		pascalName,
		pascalName,
		pascalName,
		pascalName,
	)
}

func renderModuleSettingsProviderGo(name string) string {
	header := filePurposeHeader("settings_provider.go", "settings retirement stub (post registrar finale)", "ADR-0001")

	// The ports.SettingsProvider / ports.SettingDefinition god surface was
	// deleted with the legacy registrars (ADR-0001). Module settings, when
	// needed, are contributed through the registrar-less settings path; this
	// file is a placeholder so the package keeps a stable settings home.
	return fmt.Sprintf(`package %s

%s`, name, header)
}

func renderModuleAuthzGo(name string) string {
	header := filePurposeHeader("authz.go", "canonical permission tokens exported by this module", "ADR-0009")

	return fmt.Sprintf(`package %s

%s
import "example.com/platformkit/backend-kit/security/authz"

// ModulePermissionTokens declares the canonical permission tokens exported by
// this module. A []string LITERAL is passed directly (not contracts.ModulePermissions())
// because the module-contract conformance analyzer reads these tokens via static
// AST inspection and cannot follow a function call. Keep the tokens and order in
// sync with contracts.ModulePermissions() and module.manifest.yaml.
var ModulePermissionTokens = authz.MustNormalizePermissionTokens([]string{
	%q,
	%q,
})
`, name, header, name+":view", name+":manage")
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
	b.WriteString("- `invocations.go`: fx wiring for health (group) and translations (registrar)\n")
	b.WriteString("- `surfaces.go`: declarative admin surface contribution (ADR-0001)\n")
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

// ModulePermissions lists the module's tokens in canonical [view, manage] order,
// matching authz.ModulePermissionTokens and the module.manifest.yaml permissions.
func ModulePermissions() []string {
	return []string{
		Permission%sView,
		Permission%sManage,
	}
}
`, header, pascalName, name+":view", pascalName, name+":manage", pascalName, pascalName)
}

func renderModuleRoutesGo(name string) string {
	header := filePurposeHeader("routes.go", "route-level constants exposed for cross-module reference", "ADR-0009")

	// NOTE: no ModuleEndpoints()/[]module.EndpointDefinition here. The feature-route
	// single-source-of-truth invariant (#4) requires EndpointDefinition declarations
	// to live ONLY in feature.go (via FeatureBuilder). Emitting them in contracts/
	// fails `+"`"+`make check-features`+"`"+` / module-contract-check for supported+ tiers.
	return fmt.Sprintf(`package contracts

%s
const (
	ModuleAPIBasePath = ModuleBasePath
)
`, header)
}

func renderProvidesDocGo(name string) string {
	header := filePurposeHeader("doc.go", "package docstring for the provides contract surface", "ADR-0009")

	return fmt.Sprintf(`// Package provides contains provider-side interfaces and DTOs exported by
// %s.
package provides

%s`, name, header)
}

// renderModuleSkillsYAML emits module.skills.yaml. The metadata block MUST mirror
// module.manifest.yaml exactly (name/version/description/tags) — module-contract-check
// compares the skills manifest's metadata.description against the module manifest's
// and fails on any drift. Every module needs this file or conformance fails closed.
func renderModuleSkillsYAML(name, description string, tags []string) string {
	display := moduleDisplayName(name)
	var b strings.Builder
	b.WriteString("apiVersion: platformkit.dev/v1\n")
	b.WriteString("kind: ModuleSkillsManifest\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	b.WriteString("  version: 1.0.0\n")
	b.WriteString("  description: " + yamlString(description) + "\n")
	b.WriteString("  author: platformkit Team\n")
	b.WriteString("  license: MIT\n")
	if len(tags) > 0 {
		b.WriteString("  tags:\n")
		for _, t := range tags {
			b.WriteString("    - " + t + "\n")
		}
	}
	b.WriteString("skills:\n")
	b.WriteString("  - id: " + name + ".list\n")
	b.WriteString("    name: List " + display + " records\n")
	b.WriteString("    description: " + yamlString("Read-side listing surface for "+name+".") + "\n")
	b.WriteString("    tags:\n")
	b.WriteString("      - module\n")
	b.WriteString("  - id: " + name + ".manage\n")
	b.WriteString("    name: Manage " + display + " records\n")
	b.WriteString("    description: " + yamlString("Admin-gated create and update surface for "+name+".") + "\n")
	b.WriteString("    tags:\n")
	b.WriteString("      - module\n")
	b.WriteString("      - admin\n")
	return b.String()
}

func renderModuleManifestYAML(name, description, category, resourceName string, tags, features, events, ports []string) string {
	var b strings.Builder
	b.WriteString("apiVersion: platformkit.dev/v1\n")
	b.WriteString("kind: ModuleManifest\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	// Must equal the ModuleVersion const emitted in module.go / contracts/module.go;
	// module-contract-check fails on metadata.version drift.
	b.WriteString("  version: 1.0.0\n")
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
	// ADR-0001 registrar finale: the admin surface is contributed via
	// ModuleSurfaceContribution and health via the "health_providers" fx group,
	// so they are not manifest dependencies. Translation registration remains a
	// typed port dependency. (module-normalize regenerates this projection from
	// the authored code; keep it consistent with dependencies.go.)
	b.WriteString("  dependencies:\n")
	b.WriteString("    required:\n")
	b.WriteString("      - interface: translationprovides.TranslationRegistrar\n")
	b.WriteString("        description: Register " + resourceName + " translations\n")
	b.WriteString("        version: \"*\"\n")
	b.WriteString("    optional:\n")
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

// Implements: REQ-002, REQ-016.
// Per: ADR-0017 (fx composition), ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"fmt"
	"sort"
	"strings"
)

// filePurposeHeader emits the C-14 block that NormalizeGeneratedGoSource
// moves ahead of the package clause. It produces canonical lines like:
//
//	// Implements: REQ-002.
//	// Per: ADR-0017, ADR-0029.
//	// Discipline: C-14.
//
// Refs are classified by prefix. REQ-002 is the default requirement because
// every emitted source file is part of the scaffold contract.
func filePurposeHeader(filename, description string, refs ...string) string {
	var reqs, adrs []string
	for _, r := range refs {
		r = strings.TrimSpace(r)
		switch {
		case strings.HasPrefix(r, "REQ-"):
			reqs = append(reqs, r)
		case strings.HasPrefix(r, "ADR-"):
			adrs = append(adrs, r)
		}
	}
	if len(reqs) == 0 {
		reqs = []string{"REQ-002"}
	}
	if len(adrs) == 0 {
		// At minimum, the file follows ADR-0029 (which created this header).
		adrs = []string{"ADR-0029"}
	}

	verb := "Implements"
	if strings.HasSuffix(filename, "_test.go") {
		verb = "Validates"
	}

	var b strings.Builder
	b.WriteString("// ")
	b.WriteString(verb)
	b.WriteString(": ")
	b.WriteString(strings.Join(reqs, ", "))
	b.WriteString(".\n")
	b.WriteString("// Per: ")
	b.WriteString(strings.Join(adrs, ", "))
	b.WriteString(".\n")
	b.WriteString("// Discipline: C-14.\n\n")
	b.WriteString("// ")
	b.WriteString(filename)
	b.WriteString(" — ")
	b.WriteString(description)
	b.WriteString(".\n\n")
	return b.String()
}

// generateModuleFiles produces the full canonical file set for a new
// PlatformKit business module.
func generateModuleFiles(opts ModuleOptions) []GeneratedFile {
	name := opts.Name
	description := opts.Description
	category := opts.Category
	archetype := opts.Archetype
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
		{Path: "transactions.go", Content: renderModuleTransactionsGo(name)},
		{Path: "jobs.go", Content: renderModuleJobsGo(name)},
		{Path: "metadata.go", Content: renderModuleMetadataGo(name, moduleFeatures, len(moduleEvents) > 0)},
		{Path: "dependencies.go", Content: renderModuleDependenciesGo(name, modulePorts)},
		{Path: "invocations.go", Content: renderModuleInvocationsGo(name, pascalName)},
		{Path: "surfaces.go", Content: renderModuleSurfacesGo(name, displayName, pascalName, resourceKebab)},
		{Path: "authz.go", Content: renderModuleAuthzGo(name)},
		{Path: "entity_permissions.go", Content: renderModuleEntityPermissionsGo(name)},
		{Path: "README.md", Content: renderModuleReadme(name, description, category, archetype, moduleTags, moduleFeatures)},
		{Path: "module_smoke_test.go", Content: renderModuleSmokeTestGo(name)},
		{Path: "module.manifest.yaml", Content: renderModuleManifestYAML(name, description, category, resourceName, moduleTags, moduleFeatures, moduleEvents, modulePorts)},
		{Path: "module.skills.yaml", Content: renderModuleSkillsYAML(name, description, moduleTags)},
		{Path: "contracts/module.go", Content: renderModuleContractsGo(name, description, category, moduleTags)},
		{Path: "contracts/permissions.go", Content: renderModulePermissionsGo(name, pascalName)},
		{Path: "contracts/provides/doc.go", Content: renderProvidesDocGo(name)},
	}
	if len(moduleEvents) > 0 {
		files = append(files,
			GeneratedFile{Path: "events.go", Content: renderModuleEventsGo(name, moduleEvents)},
			GeneratedFile{Path: "contracts/events.go", Content: renderModuleEventContractsGo(moduleEvents)},
		)
	}

	if archetype != "infrastructure" {
		files = append(files, GeneratedFile{
			Path:    "features/README.md",
			Content: renderFeaturesReadme(name),
		})
	}

	if needsModuleMigrations(archetype) {
		files = append(files, GeneratedFile{Path: "migrations/README.md", Content: renderMigrationsReadme(name)})
	}

	for _, featureName := range moduleFeatures {
		featureFiles := generateFeatureFiles(name, featureName, nil)
		featureFiles = append(featureFiles,
			GeneratedFile{Path: "feature_test.go", Content: generateFeatureTestCode(name, featureName)},
			GeneratedFile{Path: "e2e.go", Content: generateFeatureE2ECode(name, featureName)},
		)
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

func renderModuleTransactionsGo(name string) string {
	header := filePurposeHeader("transactions.go", "transaction and durable event boundary for module services", "ADR-0007", "ADR-0009")
	return fmt.Sprintf(`package %s

%s
import (
	"context"
	"fmt"

	"example.com/platformkit/backend-kit/app/event"
	"example.com/platformkit/backend-kit/core/crud"
)

// TransactionRunner is the module-owned entry point for state changes that
// also emit domain events. Use Run so the state write and outbox-backed event
// are committed together.
type TransactionRunner struct {
	unitOfWork *crud.UnitOfWork
}

func NewTransactionRunner(repository crud.AtomicTransactionRepository, publisher event.EventPublisher) (*TransactionRunner, error) {
	uow, err := crud.NewUnitOfWork(repository, publisher)
	if err != nil {
		return nil, err
	}
	return &TransactionRunner{unitOfWork: uow}, nil
}

func (r *TransactionRunner) Run(ctx context.Context, fn func(*crud.Transaction) error) error {
	if r == nil || r.unitOfWork == nil {
		return fmt.Errorf("%s: transaction runner is not configured")
	}
	return r.unitOfWork.Run(ctx, fn)
}
`, name, header, name)
}

func renderModuleJobsGo(name string) string {
	header := filePurposeHeader("jobs.go", "typed tenant-aware job registration helpers", "ADR-0007", "ADR-0009")
	return fmt.Sprintf(`package %s

%s
import (
	"context"
	"time"

	"example.com/platformkit/backend-kit/infrastructure/jobs"
)

// ScheduleJob is the module-level safe default for one-shot work.
func ScheduleJob(ctx context.Context, scheduler jobs.JobScheduler, eventType string, payload any, executeAt time.Time) (string, error) {
	return jobs.ScheduleOnce(ctx, scheduler, eventType, payload, executeAt)
}

// ScheduleRecurringJob is the module-level safe default. The key must be
// stable for the logical schedule, not generated per process.
func ScheduleRecurringJob(ctx context.Context, scheduler jobs.JobScheduler, key, eventType string, payload any, cronSpec string) (string, error) {
	return jobs.ScheduleRecurringWithKey(ctx, scheduler, key, eventType, payload, cronSpec)
}

func ScheduleTypedRecurringJob[T any](ctx context.Context, scheduler jobs.JobScheduler, key, eventType string, envelope jobs.Envelope, input T, cronSpec string) (string, error) {
	return jobs.ScheduleTypedRecurring(ctx, scheduler, key, eventType, envelope, input, cronSpec)
}

// NewTypedJob keeps job payload decoding, tenant restoration, deadlines, and
// idempotency validation in the platform job runtime.
func NewTypedJob[T any](definition jobs.Definition[T]) (*jobs.TypedHandler[T], error) {
	return jobs.NewTypedHandler(definition)
}

func ScheduleTypedJob[T any](ctx context.Context, scheduler jobs.JobScheduler, eventType string, envelope jobs.Envelope, input T, executeAt time.Time) (string, error) {
	return jobs.ScheduleTyped(ctx, scheduler, eventType, envelope, input, executeAt)
}
`, name, header)
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

// eventIdentifier turns a dotted event name into a stable Go identifier for
// the generated typed contract and payload. Separators are intentionally
// collapsed because event names are transport identifiers, not Go names.
func eventIdentifier(eventName string) string {
	parts := strings.FieldsFunc(eventName, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '/'
	})
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(ToPascalCase(part))
	}
	name := b.String()
	if name == "" {
		return "DeclaredEvent"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "Event" + name
	}
	return name
}

// uniqueEventIdentifiers allocates deterministic, package-safe identifiers
// for a normalized event list. It considers the complete set, so a natural
// name such as FooBar2 cannot collide with a generated suffix for FooBar.
func uniqueEventIdentifiers(events []string) []string {
	baseNames := make([]string, len(events))
	baseCounts := make(map[string]int, len(events))
	for i, eventName := range events {
		baseNames[i] = eventIdentifier(eventName)
		baseCounts[baseNames[i]]++
	}
	reservedBases := make(map[string]struct{}, len(baseCounts))
	for base := range baseCounts {
		reservedBases[base] = struct{}{}
	}

	used := make(map[string]struct{}, len(events))
	identifiers := make([]string, len(events))
	for i, base := range baseNames {
		if _, exists := used[base]; !exists {
			used[base] = struct{}{}
			identifiers[i] = base
			continue
		}

		for suffix := 2; ; suffix++ {
			candidate := fmt.Sprintf("%s%d", base, suffix)
			if _, exists := used[candidate]; exists {
				continue
			}
			if _, reserved := reservedBases[candidate]; reserved {
				continue
			}
			used[candidate] = struct{}{}
			identifiers[i] = candidate
			break
		}
	}
	return identifiers
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
	imports = append(imports, `portsurface "example.com/platformkit/ports/surface"`)
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

	// Declarative surface contribution. The fx-group value feeds the admin collector /
	// validators; the name-tagged typed provider supports interface discovery
	// without a duplicate-provide error across registrar-less modules.
	m.WithProviders(
		fx.Annotate(
			func() portsurface.Contribution { return moduleSurfaceContribution() },
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

func renderModuleMetadataGo(name string, features []string, hasEvents bool) string {
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
	eventBlock := ""
	if hasEvents {
		eventBlock = "\toptions = append(options, moduleEventOptions()...)\n"
	}

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
%s	return options
}
`, name, header, joinImports(imports), featureBlock, eventBlock)
}

func renderModuleDependenciesGo(name string, extraPorts []string) string {
	header := filePurposeHeader("dependencies.go", "cross-module dependency declarations via standard.WithDep + module.RequiresPort/OptionalPort", "ADR-0001", "ADR-0009", "ADR-0017")

	// Canonical wiring: the admin surface is contributed
	// declaratively via surface.Contribution (surfaces.go / module.go) and
	// health via the "health_providers" fx group (invocations.go). Translation
	// registration is a typed port dependency on translation_management's
	// published contract.
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`		standard.WithDep(module.RequiresPort[translationprovides.TranslationRegistrar](module.PortSpec{Purpose: "Register %s translations", Category: module.DependencyCategoryInfrastructure, SubCategory: "i18n", PreferredProvider: "translation_management"})),`, moduleResourceName(name)))
	b.WriteString("\n")

	// Extra ports the operator declared on the command line.
	for _, p := range extraPorts {
		b.WriteString(fmt.Sprintf(`		standard.WithDep(module.OptionalPort[ports.%s](module.PortSpec{Purpose: %q, Category: module.DependencyCategoryData, SubCategory: %q})),`, p, moduleDisplayName(name)+" integration through ports."+p, ToSnakeCase(p)))
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

	var b strings.Builder
	identifiers := uniqueEventIdentifiers(events)
	for i := range events {
		identifier := identifiers[i]
		b.WriteString(fmt.Sprintf("\t\tstandard.WithEventContract(contracts.%s.Contract()),", identifier))
		b.WriteString("\n")
	}

	return fmt.Sprintf(`package %s

%s
import (
	"example.com/platformkit/backend-kit/app/module/providers/standard"
	"example.com/platformkit/business-modules/%s/contracts"
)

// moduleEventOptions returns the typed event contracts this module emits.
// Durable delivery is the default; the composer derives the required
// transactional EventPublisher dependency from these declarations.
func moduleEventOptions() []standard.ComposerOption {
	return []standard.ComposerOption{
%s	}
}
`, name, header, name, b.String())
}

func renderModuleEventContractsGo(events []string) string {
	header := filePurposeHeader("events.go", "typed event contracts and starter payload schemas", "ADR-0018")
	var b strings.Builder
	identifiers := uniqueEventIdentifiers(events)
	for i, evt := range events {
		identifier := identifiers[i]
		b.WriteString(fmt.Sprintf(`// %sPayload is the versioned wire payload for %s.
// Replace the starter fields with the domain-specific contract before release.
type %sPayload struct {
	TenantID  string    `+"`json:\"tenantId\"`"+`
	Timestamp time.Time `+"`json:\"timestamp\"`"+`
}

// %s is the canonical typed declaration for %s.
var %s = port.Event[%sPayload]{
	Name:       %q,
	Version:    "1.0.0",
	Doc:        %q,
	Durability: port.EventDurabilityDurable,
}

`, identifier, evt, identifier, identifier, evt, identifier, identifier, evt, "Durable contract for "+evt+"."))
	}

	return fmt.Sprintf(`package contracts

%s
import (
	"time"

	"example.com/platformkit/ports/port"
)

%s`, header, b.String())
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
		// Canonical wiring: the admin surface is contributed
		// via surface.Contribution (surfaces.go / module.go), health via the
		// "health_providers" fx group, and translations via the translation
		// registrar port.

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
	header := filePurposeHeader("surfaces.go", "declarative admin surface contribution", "ADR-0001", "ADR-0009", "ADR-0017")

	return fmt.Sprintf(
		`package %s

%s
import (
	contracts "example.com/platformkit/business-modules/%s/contracts"
	"example.com/platformkit/business-modules/ports"
	portsurface "example.com/platformkit/ports/surface"
)

// moduleSurfaceContribution registers this module's admin page. The route ID
// equals the module name, which is also the canonical section-dispatch key.
// PagePattern stays Unknown because the default entity-table renderer handles
// RegisterEntity surfaces until the module supplies an intentional custom view.
func moduleSurfaceContribution() portsurface.Contribution {
	return ports.CanonicalizeAdminSurfaceContribution(portsurface.Contribution{
		ModuleID: ModuleName,
		Routes: []portsurface.Route{
			{
				ID:             ModuleName,
				Path:           "/admin/%s",
				Title:          portsurface.Text{Fallback: %q},
				NavLabel:       portsurface.Text{Fallback: %q},
				Icon:           "box",
				Order:          100,
				PagePattern:    portsurface.PagePatternUnknown,
				CapabilityTags: []string{contracts.Permission%sRead},
				Targets:        []portsurface.Target{portsurface.TargetAdmin},
			},
		},
	})
}

// %sSurfaceContributionProvider exposes the module's typed surface contribution.
type %sSurfaceContributionProvider struct {
	contribution portsurface.Contribution
}

func New%sSurfaceContributionProvider() *%sSurfaceContributionProvider {
	return &%sSurfaceContributionProvider{contribution: moduleSurfaceContribution()}
}

func (p *%sSurfaceContributionProvider) GetSurfaceContribution() portsurface.Contribution {
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
`, name, header, name+":read", name+":manage")
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
`, name, header, name)
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
	b.WriteString("- `transactions.go`: transaction-bound state and durable event boundary\n")
	b.WriteString("- `jobs.go`: typed tenant-aware job and stable schedule helpers\n")
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
	b.WriteString("- State changes that emit events should use `TransactionRunner` from `transactions.go`.\n")
	b.WriteString("- Every emitted event must be declared via `standard.WithEventContract` (ADR-0018).\n")
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

func renderModulePermissionsGo(name, pascalName string) string {
	header := filePurposeHeader("permissions.go", "permission token constants exposed by this module", "ADR-0009")

	return fmt.Sprintf(`package contracts

%s
const (
	Permission%sRead   = %q
	Permission%sManage = %q
)

// ModulePermissions lists the module's tokens in canonical [read, manage] order,
// matching authz.ModulePermissionTokens and the module.manifest.yaml permissions.
func ModulePermissions() []string {
	return []string{
		Permission%sRead,
		Permission%sManage,
	}
}
`, header, pascalName, name+":read", pascalName, name+":manage", pascalName, pascalName)
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
	// surface.Contribution and health via the "health_providers" fx group,
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
		b.WriteString("        description: " + yamlString(moduleDisplayName(name)+" integration through ports."+p) + "\n")
		b.WriteString("        version: \"*\"\n")
	}
	if len(features) > 0 {
		b.WriteString("  features:\n")
		for _, f := range features {
			b.WriteString("    - id: " + f + "\n")
			b.WriteString("      name: " + moduleDisplayName(f) + "\n")
			b.WriteString("      description: " + yamlString(moduleDisplayName(f)+" feature for "+name+".") + "\n")
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
			b.WriteString("      description: " + yamlString("Durable contract for "+e+".") + "\n")
			b.WriteString("      durability: durable\n")
		}
	} else {
		b.WriteString("  events: []\n")
	}
	b.WriteString("  permissions:\n")
	b.WriteString("    permissions:\n")
	b.WriteString("      - token: " + name + ":read\n")
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

func renderFeaturesReadme(name string) string {
	return fmt.Sprintf(`# %s features

Add one subdirectory per feature. Every feature owns:

- `+"`feature.go`"+`
- `+"`feature_test.go`"+`
- `+"`e2e.go`"+`

Generate `+"`service.go`"+` only when named use cases exist. Add a handler only
with concrete endpoint definitions; feature.go remains the route metadata source
of truth. Add a custom section renderer only with a complete, domain-specific
view contract.

Wire new feature constructors into `+"`metadata.go`"+` via `+"`standard.WithFeatures(...)`"+`.
`, name)
}

func renderMigrationsReadme(name string) string {
	return fmt.Sprintf(`# %s migrations

Store append-only Goose migrations here using `+"`NNNN_name.up.sql`"+` and
`+"`NNNN_name.down.sql`"+` pairs. Generate the first real entity migration with
an explicit sequence; never reserve a number with an empty migration and never
edit a committed migration.
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

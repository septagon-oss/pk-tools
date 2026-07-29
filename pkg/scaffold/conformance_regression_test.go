// Validates: REQ-002, REQ-016.
// Per: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestScaffoldedModuleIsBornConformant locks the module templates to the
// platformkit module-contract conformance rules. Each assertion corresponds to a
// real finding that module-contract-check raised against scaffolded modules; if a
// template regresses, this test fails HERE (fast, local) instead of days later in
// a downstream `make check-module-contracts` run. Keep this in sync with the
// conformance analyzer in platformkit-business-modules.
func TestScaffoldedModuleIsBornConformant(t *testing.T) {
	// A colon in the description stresses YAML quoting and the manifest/skills
	// description-match rule.
	const (
		name = "demo_widget_management"
		desc = "Demo widgets: gizmos, gadgets, and their lifecycle"
	)
	result := mustGenerateModule(t, ModuleOptions{
		Name:        name,
		Description: desc,
		Category:    "workspace",
		Archetype:   "service",
		Features:    []string{"widgets"},
		Tags:        []string{"workspace"},
	})

	files := map[string]string{}
	for _, f := range result.Files {
		files[f.Path] = f.Content
	}
	get := func(path string) string {
		t.Helper()
		c, ok := files[path]
		if !ok {
			t.Fatalf("scaffold did not generate %s", path)
		}
		return c
	}

	// Invariant #8 — modern dependency declaration; removed legacy APIs must
	// never be emitted (they do not compile against current backend-kit, and
	// standard.WithDep belongs to the retired composer lane).
	for path, content := range files {
		if strings.Contains(content, "WithCategorizedDep") {
			t.Errorf("%s emits removed standard.WithCategorizedDep; declare platform.PortDecl entries", path)
		}
		if strings.Contains(content, "standard.WithDep") {
			t.Errorf("%s emits legacy standard.WithDep; declare platform.PortDecl entries", path)
		}
		if strings.Contains(content, ":view") {
			t.Errorf("%s emits retired permission verb view; use read", path)
		}
	}
	if deps := get("dependencies.go"); !strings.Contains(deps, "func moduleRequiredPorts() []platform.PortDecl") {
		t.Errorf("dependencies.go must declare moduleRequiredPorts() []platform.PortDecl; got:\n%s", deps)
	}

	// authz: conformance reads tokens via static AST, so a []string LITERAL must be
	// passed to MustNormalizePermissionTokens — not a function call.
	authz := get("authz.go")
	if !strings.Contains(authz, "MustNormalizePermissionTokens([]string{") {
		t.Errorf("authz.go must pass a []string literal to MustNormalizePermissionTokens; got:\n%s", authz)
	}
	if strings.Contains(authz, "MustNormalizePermissionTokens(contracts.ModulePermissions") {
		t.Errorf("authz.go must not pass contracts.ModulePermissions() into MustNormalizePermissionTokens (analyzer cannot follow a call)")
	}

	// Invariant #4 — route single source of truth: no alias-only contracts file.
	if _, exists := files["contracts/routes.go"]; exists {
		t.Error("scaffold must not generate contracts/routes.go; route truth lives in feature.go")
	}

	// module.skills.yaml must exist and its metadata.description must EXACTLY match
	// module.manifest.yaml (the conformance analyzer compares them).
	skills := get("module.skills.yaml")
	if !strings.Contains(skills, "kind: ModuleSkillsManifest") {
		t.Errorf("module.skills.yaml missing or wrong kind:\n%s", skills)
	}
	manifest := get("module.manifest.yaml")
	manifestDesc := descriptionLine(manifest)
	skillsDesc := descriptionLine(skills)
	if manifestDesc == "" {
		t.Fatalf("manifest has no description line")
	}
	if manifestDesc != skillsDesc {
		t.Errorf("skills/manifest description drift:\n manifest: %q\n skills:   %q", manifestDesc, skillsDesc)
	}

	// manifest metadata.version must equal the ModuleVersion const (1.0.0).
	if !strings.Contains(manifest, "  version: 1.0.0\n") {
		t.Errorf("module.manifest.yaml metadata.version must be 1.0.0; got:\n%s", manifest)
	}
	if mod := get("module.go"); !strings.Contains(mod, `ModuleVersion     = "1.0.0"`) {
		t.Errorf("module.go ModuleVersion must be 1.0.0 to match the manifest")
	}
}

// TestScaffoldedModuleUsesDescriptorComposition locks module.go to the
// descriptor composition lane: pkdef.Define(pkdef.StandardRuntime(
// platform.ModuleDescriptor{...})). The internal repository enforces an
// only-shrinking ledger of legacy standard.NewComposer call sites
// (composer_convergence.go, ADR-0047), so a newly scaffolded module that
// emits NewComposer fails `make check-composer-convergence` on arrival.
func TestScaffoldedModuleUsesDescriptorComposition(t *testing.T) {
	result := mustGenerateModule(t, ModuleOptions{
		Name:        "drift_probe_management",
		Description: "Scaffold drift probe.",
		Category:    "workspace",
		Archetype:   "service",
		Features:    []string{"catalog_sync"},
	})

	files := map[string]string{}
	for _, f := range result.Files {
		files[f.Path] = f.Content
	}
	moduleGo, ok := files["module.go"]
	if !ok {
		t.Fatal("scaffold did not produce module.go")
	}

	for _, needle := range []string{
		"pkdef.Define(",
		"platform.ModuleDescriptor{",
		"platformfx.DescriptorModule",
		"NewRuntimeForEnvironment",
	} {
		if !strings.Contains(moduleGo, needle) {
			t.Errorf("module.go must contain %q — descriptor composition shape", needle)
		}
	}
	// Service archetype: migrations stay descriptor-owned.
	for _, needle := range []string{
		"//go:embed migrations/*",
		"Migrations: platform.SQLAssets{FS: migrationsFS, Dir: \"migrations\"},",
	} {
		if !strings.Contains(moduleGo, needle) {
			t.Errorf("module.go must contain %q — descriptor-owned migrations", needle)
		}
	}
	for path, content := range files {
		if strings.Contains(content, "standard.NewComposer") {
			t.Errorf("%s emits legacy standard.NewComposer — new modules must be descriptor-composed", path)
		}
	}

	// One scaffold run must author ONE consistent feature record: the
	// Descriptor literal in module.go and the manifest features block agree
	// on name, description, version, and category (developer_portal is the
	// exemplar: display-style Name, Category = module category).
	for _, needle := range []string{
		`Name:        "Catalog Sync",`,
		`Description: "Catalog Sync feature for drift_probe_management.",`,
		"Version:     ModuleVersion,",
		"Category:    ModuleCategory,",
	} {
		if !strings.Contains(moduleGo, needle) {
			t.Errorf("module.go feature descriptor must contain %q — must match the manifest features block", needle)
		}
	}
	if strings.Contains(moduleGo, "Category:    ModuleName,") {
		t.Error("module.go feature descriptor must use Category: ModuleCategory (developer_portal exemplar), not ModuleName")
	}
	manifest, ok := files["module.manifest.yaml"]
	if !ok {
		t.Fatal("scaffold did not produce module.manifest.yaml")
	}
	for _, needle := range []string{
		"      name: Catalog Sync\n",
		"      description: Catalog Sync feature for drift_probe_management.\n",
		"      version: 1.0.0\n",
		"      category: workspace\n",
	} {
		if !strings.Contains(manifest, needle) {
			t.Errorf("module.manifest.yaml features block must contain %q — must match the module.go descriptor literal", needle)
		}
	}
	if strings.Contains(manifest, "0.1.0") {
		t.Error("module.manifest.yaml must not version features 0.1.0 while module.go authors ModuleVersion")
	}

	// Registry archetype WITH features: no migrations embed, still
	// descriptor-composed, reconciled feature naming, and the
	// no-migrations+features rendering must parse.
	registry := mustGenerateModule(t, ModuleOptions{
		Name:        "probe_registry",
		Description: "Registry probe.",
		Category:    "workspace",
		Archetype:   "registry",
		Features:    []string{"catalog_sync"},
	})
	registryModuleGo := ""
	for _, f := range registry.Files {
		if f.Path == "module.go" {
			registryModuleGo = f.Content
		}
	}
	if registryModuleGo == "" {
		t.Fatal("registry scaffold did not produce module.go")
	}
	if !strings.Contains(registryModuleGo, "pkdef.Define(") {
		t.Error("registry module.go must be descriptor-composed")
	}
	if strings.Contains(registryModuleGo, "embed.FS") || strings.Contains(registryModuleGo, "platform.SQLAssets") {
		t.Error("registry module.go must not embed migrations")
	}
	if !strings.Contains(registryModuleGo, `Name:        "Catalog Sync",`) {
		t.Error("registry module.go feature descriptor must carry the reconciled display Name")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "module.go", registryModuleGo, parser.AllErrors); err != nil {
		t.Errorf("registry module.go (features without migrations) does not parse: %v", err)
	}
}

// descriptionLine returns the first top-level "  description: ..." line (2-space
// indent = metadata block), trimmed.
func descriptionLine(yaml string) string {
	for line := range strings.SplitSeq(yaml, "\n") {
		if after, ok := strings.CutPrefix(line, "  description: "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// Validates: REQ-002, REQ-016.
// Per: ADR-0017 (composition through dependency injection), ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import (
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

	// Invariant #8 — modern dependency declaration; the removed legacy API must
	// never be emitted (it does not compile against current backend-kit).
	for path, content := range files {
		if strings.Contains(content, "WithCategorizedDep") {
			t.Errorf("%s emits removed standard.WithCategorizedDep; use WithDep(RequiresPort[...])", path)
		}
		if strings.Contains(content, ":view") {
			t.Errorf("%s emits retired permission verb view; use read", path)
		}
	}
	if deps := get("dependencies.go"); !strings.Contains(deps, "standard.WithDep(module.RequiresPort[") {
		t.Errorf("dependencies.go must use standard.WithDep(module.RequiresPort[...]); got:\n%s", deps)
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

// Implements: REQ-002.
// Per: ADR-0029 (file purpose declaration).
// Discipline: C-14.

package scaffold

import "strings"

const (
	defaultBackendKitImportRoot      = "example.com/platformkit/backend-kit"
	defaultBusinessModulesImportRoot = "example.com/platformkit/business-modules"
	defaultFrontendKitImportRoot     = "example.com/platformkit/frontend-kit"
	defaultPortsImportRoot           = "example.com/platformkit/ports"
	defaultTestsImportRoot           = "example.com/platformkit/tests"
)

// ImportProfile describes the repository roots used in generated Go source.
// The scaffold engine is OSS-neutral by default; private distributions pass a
// profile from their adapter layer so generated code keeps matching that
// workspace without baking private roots into this package.
type ImportProfile struct {
	BackendKit      string `json:"backendKit,omitempty"`
	BusinessModules string `json:"businessModules,omitempty"`
	FrontendKit     string `json:"frontendKit,omitempty"`
	Ports           string `json:"ports,omitempty"`
	Tests           string `json:"tests,omitempty"`
}

// DefaultImportProfile returns the neutral roots emitted when callers do not
// provide their own workspace profile.
func DefaultImportProfile() ImportProfile {
	return ImportProfile{
		BackendKit:      defaultBackendKitImportRoot,
		BusinessModules: defaultBusinessModulesImportRoot,
		FrontendKit:     defaultFrontendKitImportRoot,
		Ports:           defaultPortsImportRoot,
		Tests:           defaultTestsImportRoot,
	}
}

func (p ImportProfile) normalized() ImportProfile {
	defaults := DefaultImportProfile()
	if strings.TrimSpace(p.BackendKit) == "" {
		p.BackendKit = defaults.BackendKit
	}
	if strings.TrimSpace(p.BusinessModules) == "" {
		p.BusinessModules = defaults.BusinessModules
	}
	if strings.TrimSpace(p.FrontendKit) == "" {
		p.FrontendKit = defaults.FrontendKit
	}
	if strings.TrimSpace(p.Ports) == "" {
		p.Ports = defaults.Ports
	}
	if strings.TrimSpace(p.Tests) == "" {
		p.Tests = defaults.Tests
	}
	return p
}

func applyImportProfile(content string, profile ImportProfile) string {
	profile = profile.normalized()
	replacer := strings.NewReplacer(
		defaultBackendKitImportRoot, profile.BackendKit,
		defaultBusinessModulesImportRoot, profile.BusinessModules,
		defaultFrontendKitImportRoot, profile.FrontendKit,
		defaultPortsImportRoot, profile.Ports,
		defaultTestsImportRoot, profile.Tests,
	)
	return replacer.Replace(content)
}

func applyImportProfileToFiles(files []GeneratedFile, profile ImportProfile) []GeneratedFile {
	out := make([]GeneratedFile, len(files))
	for i, file := range files {
		out[i] = file
		out[i].Content = applyImportProfile(file.Content, profile)
	}
	return out
}

func applyImportProfileToRegistrationCode(reg map[string]string, profile ImportProfile) map[string]string {
	out := make(map[string]string, len(reg))
	for key, value := range reg {
		out[key] = applyImportProfile(value, profile)
	}
	return out
}

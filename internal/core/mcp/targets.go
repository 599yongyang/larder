package mcp

// Target is an agent config file an MCP server can be written to.
type Target struct {
	Label  string
	Path   string
	Format string
}

// Targets returns every agent config file the installer can write. The list
// matches the scanner's config specs; files may not exist yet.
func Targets() []Target {
	specs := configs()
	targets := make([]Target, 0, len(specs))
	for _, spec := range specs {
		targets = append(targets, Target{Label: spec.label, Path: spec.path, Format: spec.format})
	}
	return targets
}

// WritableTargets drops configs whose schema is not supported by the installer
// yet, so the UI never offers a write that the client would not read.
func WritableTargets() []Target {
	targets := Targets()
	out := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.Label == "Continue" {
			continue
		}
		out = append(out, target)
	}
	return out
}

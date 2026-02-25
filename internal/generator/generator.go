package generator

// GeneratorConfig holds common settings used by all generators.
type GeneratorConfig struct {
	DaemonPort int    // Port the daemon will listen on
	Token      string // Auth token (auto-generated if empty)
}

// Generator is the interface for AI tool-specific file generators.
// Each AI tool (Claude Code, Cursor, etc.) implements this interface
// to produce its own rules/skills/hooks files.
type Generator interface {
	// Name returns the human-readable name of the AI tool.
	Name() string

	// Generate creates all necessary files in the project directory.
	// It should be idempotent — safe to run multiple times.
	Generate() error
}

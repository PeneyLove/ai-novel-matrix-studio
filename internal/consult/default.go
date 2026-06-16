package consult

// NewDefaultEngine creates a consultation engine with all built-in strategies
// pre-registered: outline completeness, character consistency, plot structure,
// and pacing health.
func NewDefaultEngine() *Engine {
	return NewEngine([]Strategy{
		NewOutlineValidator(nil),
		NewCharacterAnalyzer(),
		NewPlotStructureAnalyzer(),
		NewPacingAnalyzer(),
	})
}

// QuickConsult runs all default strategies against the given source text and
// returns a formatted report string. This is the simplest entry point for
// callers that just want to "run a consult and show the result".
func QuickConsult(subject, src string) string {
	engine := NewDefaultEngine()
	report := engine.Consult(subject, src)
	return FormatReport(report)
}

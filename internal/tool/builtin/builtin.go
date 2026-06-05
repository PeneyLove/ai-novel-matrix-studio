// Package builtin registers the novel-agent tool set at compile time.
// Each tool self-registers via init() → tool.RegisterBuiltin().
package builtin

// Blank imports trigger all init() functions in the sub-files.
// Actual tool implementations are in the files below.

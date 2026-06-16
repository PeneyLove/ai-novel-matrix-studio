package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/event"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

// PrefixShape hashes the portions of the request prefix that influence
// provider-side prompt-cache reuse. Comparing snapshots across turns
// lets us explain *why* a cache miss happened.
type PrefixShape struct {
	SystemHash        string
	ToolsHash         string
	PrefixHash        string
	LogRewriteVersion int
	ToolSchemaTokens  int

	// sysTokenEstimate and toolTokenEstimate are stored alongside hashes so
	// CompareShape can compute cache-block alignment diagnostics without
	// re-marshalling the full schemas. Set by CaptureShape. Zero before the
	// first turn.
	sysTokenEstimate  int
	toolTokenEstimate int
}

// CacheDiagnostics is a type alias for event.CacheDiagnostics so the agent
// can construct and compare diagnostics without importing event itself in
// every call site, while still assigning to event.Event.CacheDiagnostics.
type CacheDiagnostics = event.CacheDiagnostics

func shortHash(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

// CaptureShape takes a snapshot of the current prefix state.
func CaptureShape(systemPrompt string, schemas []provider.ToolSchema, rewriteVersion int) PrefixShape {
	normalizedSchemas := normalizeToolSchemas(schemas)
	toolsJSON, _ := json.Marshal(normalizedSchemas)
	sysTokens := estimateTokens(systemPrompt)
	toolTokens := estimateTokens(string(toolsJSON))
	return PrefixShape{
		SystemHash: shortHash(systemPrompt),
		ToolsHash:  shortHash(string(toolsJSON)),
		PrefixHash: shortHash(map[string]interface{}{
			"system": systemPrompt,
			"tools":  string(toolsJSON),
		}),
		LogRewriteVersion: rewriteVersion,
		ToolSchemaTokens:  toolTokens,
		// Stash raw estimates so CompareShape can build alignment info without
		// re-marshalling the schemas or re-scanning the system prompt.
		sysTokenEstimate:  sysTokens,
		toolTokenEstimate: toolTokens,
	}
}

func normalizeToolSchemas(schemas []provider.ToolSchema) []provider.ToolSchema {
	out := make([]provider.ToolSchema, len(schemas))
	copy(out, schemas)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Description != out[j].Description {
			return out[i].Description < out[j].Description
		}
		return string(out[i].Parameters) < string(out[j].Parameters)
	})
	return out
}

// CompareShape returns diagnostics describing what changed between two shapes.
func CompareShape(prev, cur PrefixShape, usage *provider.Usage) CacheDiagnostics {
	reasons := []string{}
	if prev.SystemHash != "" && prev.SystemHash != cur.SystemHash {
		reasons = append(reasons, "system")
	}
	if prev.ToolsHash != "" && prev.ToolsHash != cur.ToolsHash {
		reasons = append(reasons, "tools")
	}
	if prev.LogRewriteVersion != cur.LogRewriteVersion {
		reasons = append(reasons, "log_rewrite")
	}
	var miss, hit int
	if usage != nil {
		miss = usage.CacheMissTokens
		hit = usage.CacheHitTokens
	}

	// Build alignment info from the current shape's token estimates.
	var alignInfo *event.CacheAlignmentInfo
	totalTokens := cur.sysTokenEstimate + cur.toolTokenEstimate
	if totalTokens > 0 {
		block := totalTokens / cacheBlockGranularity
		remainder := totalTokens % cacheBlockGranularity
		wastePct := 0.0
		if remainder > 0 && totalTokens > cacheBlockGranularity {
			wastePct = float64(remainder) / float64(totalTokens) * 100
		}
		// Round to one decimal.
		wastePct = float64(int(wastePct*10)) / 10
		alignInfo = &event.CacheAlignmentInfo{
			TotalTokens:     totalTokens,
			BlocksConsumed:  block,
			LastBlockFill:   remainder,
			ShortfallToFill: cacheBlockGranularity - remainder,
			Aligned:         remainder == 0,
			WastePercent:    wastePct,
		}
		// Even when the prefix hash is stable, a partial last block means the
		// provider spends a partial block on every turn — note alignment as a
		// secondary efficiency signal so the user sees it in the UI.
		if len(reasons) == 0 && !alignInfo.Aligned && alignInfo.BlocksConsumed > 1 {
			reasons = append(reasons, "alignment")
		}
	}

	return CacheDiagnostics{
		PrefixHash:          cur.PrefixHash,
		PrefixChanged:       len(reasons) > 0,
		PrefixChangeReasons: reasons,
		SystemHash:          cur.SystemHash,
		ToolsHash:           cur.ToolsHash,
		LogRewriteVersion:   cur.LogRewriteVersion,
		ToolSchemaTokens:    cur.ToolSchemaTokens,
		CacheMissTokens:     miss,
		CacheHitTokens:      hit,
		CacheAlignment:      alignInfo,
	}
}

// estimateTokens gives a rough token count from byte length.
// A proper tokenizer would be more accurate, but for diagnostic
// purposes a byte-based estimate is sufficient and zero-alloc.
func estimateTokens(s string) int {
	// ~4 chars per token is a workable heuristic for code-heavy JSON.
	if len(s) == 0 {
		return 0
	}
	return len(s) / 4
}

// SchemaTokenCosts returns per-tool token cost estimates for display.
func SchemaTokenCosts(schemas []provider.ToolSchema) []ToolSchemaCost {
	out := make([]ToolSchemaCost, 0, len(schemas))
	for _, s := range schemas {
		b, _ := json.Marshal(s)
		out = append(out, ToolSchemaCost{Name: s.Name, Tokens: estimateTokens(string(b))})
	}
	return out
}

// ToolSchemaCost is a per-tool token cost estimate for diagnostic display.
type ToolSchemaCost struct {
	Name   string
	Tokens int
}

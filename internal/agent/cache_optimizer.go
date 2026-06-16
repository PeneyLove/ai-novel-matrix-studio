package agent

import (
	"fmt"
	"math"
	"strings"
)

// cacheBlockGranularity is the granularity at which provider-side auto-cache
// operates. DeepSeek's context cache uses 64-token blocks; other providers
// (OpenAI, MiMo) have coarser or blockless caching but still benefit from
// prefix stability rather than exact alignment.
const cacheBlockGranularity = 64

// CacheAlignment returns diagnostics about how the current prefix aligns with
// the provider's cache block boundary, so a frontend or diagnostic report can
// suggest system-prompt tweaks that improve cache efficiency (keeping the
// prefix on a block boundary avoids wasting a partial block on every turn).
func CacheAlignment(systemTokens, toolSchemaTokens int) CacheAlignmentReport {
	total := systemTokens + toolSchemaTokens
	block := total / cacheBlockGranularity
	remainder := total % cacheBlockGranularity
	fill := 0
	if remainder > 0 {
		fill = cacheBlockGranularity - remainder
	}
	// Waste ratio: the fraction of the last block consumed by prefix content
	// that is just padding from the cache's perspective. Every turn sends this
	// partial block as a cache miss — reducing it improves hit ratio.
	wastePct := 0.0
	if remainder > 0 && total > cacheBlockGranularity {
		wastePct = float64(remainder) / float64(total) * 100
	}
	return CacheAlignmentReport{
		SystemTokens:     systemTokens,
		ToolSchemaTokens: toolSchemaTokens,
		TotalTokens:      total,
		BlocksConsumed:   block + 1,
		LastBlockFill:    remainder,
		ShortfallToAlign: fill,
		WastePercent:     math.Round(wastePct*10) / 10,
		Aligned:          remainder == 0,
	}
}

// CacheAlignmentReport describes how the cacheable prefix aligns with the
// provider's cache block boundaries.
type CacheAlignmentReport struct {
	SystemTokens     int     // tokens consumed by the system prompt
	ToolSchemaTokens int     // tokens consumed by tool schemas
	TotalTokens      int     // sum of the above
	BlocksConsumed   int     // how many cache blocks the prefix occupies
	LastBlockFill    int     // tokens in the last (partial) block (0 = perfect alignment)
	ShortfallToAlign int     // padding tokens needed to fill the last block
	WastePercent     float64 // percentage of total tokens wasted as partial-block overhead
	Aligned          bool    // true when the prefix ends on a cache block boundary
}

// String returns a human-readable summary of the alignment report.
func (r CacheAlignmentReport) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("prefix: %d tokens (%d system + %d tools) = %d cache blocks",
		r.TotalTokens, r.SystemTokens, r.ToolSchemaTokens, r.BlocksConsumed))
	if r.Aligned {
		b.WriteString(" [perfect alignment]")
	} else {
		b.WriteString(fmt.Sprintf(" [last block: %d/%d, need %d more tokens to align]",
			r.LastBlockFill, cacheBlockGranularity, r.ShortfallToAlign))
	}
	return b.String()
}

// AlignSystemPrompt returns a modified system prompt that has been padded with
// cache-friendly whitespace comments to align the total prefix to the nearest
// cache block boundary. The padding is appended as a single-line comment so
// it is semantically invisible to the model while consuming tokens.
// padChar is the comment prefix to use (e.g. "// " for Go-style, "# " for TOML).
//
// NOTE: This is a best-effort heuristic based on token estimation (~4 chars/token).
// Real tokenization varies by model. Use the diagnostic output to verify.
func AlignSystemPrompt(prompt string, toolSchemaTokens int, padChar string) string {
	sysTokens := estimateTokens(prompt)
	report := CacheAlignment(sysTokens, toolSchemaTokens)
	if report.Aligned || report.ShortfallToAlign <= 0 {
		return prompt
	}
	// Each padding token takes ~4 characters. We pad with comment lines.
	padTokens := report.ShortfallToAlign
	padChars := padTokens * 4 // rough estimate
	pad := "\n" + padChar + strings.Repeat(" ", padChars-3)
	return prompt + pad
}

// CacheHitRate computes the aggregate cache hit rate from the session counters.
// Returns 0 when no data is available.
func CacheHitRate(hit, miss int) float64 {
	total := hit + miss
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total) * 100
}

// CacheHealth returns a human-readable assessment of the observed cache
// performance, suggesting configuration adjustments when the rate is poor.
func CacheHealth(hitRate float64, compactRatio float64, tailTokens int) string {
	switch {
	case hitRate >= 70:
		return fmt.Sprintf("cache hit rate is excellent (%.0f%%). No adjustments needed.", hitRate)
	case hitRate >= 40:
		return fmt.Sprintf("cache hit rate is moderate (%.0f%%). Consider increasing compact tail budget (currently %d) or raising compact ratio (currently %.0f%%) to reduce compaction frequency.",
			hitRate, tailTokens, compactRatio*100)
	case hitRate > 0:
		return fmt.Sprintf("cache hit rate is poor (%.0f%%). The system prompt or tool schemas may be changing between turns, or compaction is too aggressive. Verify that system prompt and tool list are stable. Consider raising tail budget above %d.",
			hitRate, tailTokens)
	default:
		return "no cache data available yet."
	}
}

package agent

import (
	"context"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

// WarmPrefix sends a minimal warm-up request to the provider so its
// asynchronous cache population starts before the first real turn. This
// transforms a cold-start first turn (0% cache hit) into a warm turn with
// partial hits. The warm-up uses a small max_tokens to minimise cost.
//
// The warm-up request mirrors the real request shape (system prompt + tools),
// so the cache prefix is identical. It sends an empty user message that will
// be answered with a single token, discarded.
//
// WarmPrefix is safe to call multiple times; subsequent calls are no-ops
// (the provider's cache is already populated from the first one).
func (a *Agent) WarmPrefix(ctx context.Context) {
	if a == nil || a.prov == nil {
		return
	}
	if a.warmed.Load() {
		return
	}

	// Build a minimal request with the same prefix as a real turn.
	schemas := a.tools.Schemas()
	req := provider.Request{
		Messages:    a.session.Messages,
		Tools:       schemas,
		Temperature: a.temperature,
		MaxTokens:   1, // single token — just enough to trigger cache fill
	}

	// Use a short timeout so warmup doesn't block startup for long.
	warmCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ch, err := a.prov.Stream(warmCtx, req)
	if err != nil {
		// Warmup is best-effort; failure is not an error.
		return
	}

	// Drain the stream and discard.
	for range ch {
	}

	a.warmed.Store(true)
}

// IsWarmed reports whether a warm-up request has been sent.
func (a *Agent) IsWarmed() bool {
	return a.warmed.Load()
}

package model

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/penney-101/ai-novel-agent/internal/skill"
)

// Router maps creation stages to model clients based on a skill's model_bindings.
// It provides automatic fallback when the primary model is unavailable.
type Router struct {
	mu        sync.RWMutex
	clients   map[string]Client       // provider name → client
	fallback  string                  // fallback provider name
	endpoints map[string]Config       // provider → full config
}

// NewRouter creates a Router from a map of provider→Config.
// The fallback provider is used when the primary model for a stage is not configured.
func NewRouter(configs map[string]Config, fallback string) (*Router, error) {
	r := &Router{
		clients:   make(map[string]Client, len(configs)),
		fallback:  fallback,
		endpoints: configs,
	}

	for provider, cfg := range configs {
		// Only attempt to create if we can get an API key
		if cfg.APIKey == "" {
			fmt.Fprintf(os.Stderr, "[model] skipping %s: no API key configured\n", provider)
			continue
		}
		client := NewClient(cfg)
		if client != nil {
			r.clients[provider] = NewRetryableClient(client, cfg.RetryTimes)
		}
	}

	if len(r.clients) == 0 {
		return nil, fmt.Errorf("model: no clients could be initialized from the provided configs")
	}

	// Ensure fallback exists, or pick the first available
	if _, ok := r.clients[fallback]; !ok {
		old := fallback
		for name := range r.clients {
			fallback = name
			break
		}
		fmt.Fprintf(os.Stderr, "[model] fallback %q not configured, using %q\n", old, fallback)
	}
	r.fallback = fallback

	return r, nil
}

// GetClientForStage returns a Client for the given skill and stage.
// Never returns nil — always falls back if the primary model is unavailable.
// Satisfies property P2: model routing completeness.
func (r *Router) GetClientForStage(sk *skill.Skill, stage string) Client {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider := sk.ModelFor(stage)
	if provider == "" {
		fmt.Fprintf(os.Stderr, "[model] no provider bound for stage %q, falling back to %q\n", stage, r.fallback)
		return r.clients[r.fallback]
	}

	client, ok := r.clients[provider]
	if !ok {
		fmt.Fprintf(os.Stderr, "[model] provider %q for stage %q not configured, falling back to %q\n", provider, stage, r.fallback)
		return r.clients[r.fallback]
	}
	return client
}

// GetClient returns a client by provider name directly.
func (r *Router) GetClient(provider string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[provider]
	if !ok {
		return nil, fmt.Errorf("model: no client configured for provider %q", provider)
	}
	return client, nil
}

// Generate is a convenience method that resolves the client for a stage and calls Generate.
func (r *Router) Generate(ctx context.Context, sk *skill.Skill, stage, systemPrompt, userPrompt string) (string, error) {
	client := r.GetClientForStage(sk, stage)
	return client.Generate(ctx, systemPrompt, userPrompt)
}

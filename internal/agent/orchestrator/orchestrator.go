// Package orchestrator provides the multi-agent task dispatcher. It sends
// instructions to isolated auxiliary-agent sandboxes, collects their output,
// and writes structured results into the shared novel-writer cache. The
// orchestrator itself holds zero business context — it is a pure routing and
// coordination layer that never touches the main writer agent's conversation
// history.
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/agent/sandbox"
	novelcache "github.com/PeneyLove/ai-novel-matrix-studio/internal/cache/novel"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/event"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

// ModelCaller is the interface the orchestrator needs to call a model.
// The boot layer provides a concrete provider.Provider implementation;
// tests provide a fake.
type ModelCaller interface {
	// Send submits messages and returns the collected text output.
	// Tools schemas may be nil for auxiliary tasks that only need text.
	// ctx controls cancellation and the deadline for the whole call.
	Send(ctx context.Context, req provider.Request) (text string, usage *provider.Usage, err error)
}

// defaultCaller adapts a provider.Provider to the ModelCaller interface
// by streaming and collecting the full text.
type defaultCaller struct {
	prov        provider.Provider
	temperature float64
	sink        event.Sink
}

func (c *defaultCaller) Send(ctx context.Context, req provider.Request) (string, *provider.Usage, error) {
	req.Temperature = c.temperature
	ch, err := c.prov.Stream(ctx, req)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	var lastUsage *provider.Usage
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkText:
			b.WriteString(chunk.Text)
		case provider.ChunkReasoning:
			// reasoning is not collected for auxiliary agents
			if c.sink != nil {
				c.sink.Emit(event.Event{Kind: event.Reasoning, Text: chunk.Text})
			}
		case provider.ChunkUsage:
			lastUsage = chunk.Usage
		case provider.ChunkError:
			return b.String(), lastUsage, chunk.Err
		}
	}
	return b.String(), lastUsage, nil
}

// Orchestrator is the task dispatcher for multi-agent collaboration.
// It does NOT hold business context — only references to the sandbox
// manager, model caller, and the shared asset cache.
type Orchestrator struct {
	mgr        *sandbox.SandboxManager
	caller     ModelCaller
	cache      *novelcache.AssetCache
	temperature float64
	timeout     time.Duration
}

// Options configures an Orchestrator.
type Options struct {
	// Provider is the model backend. Required.
	Provider provider.Provider
	// AssetCache is the shared novel-writing asset cache. Required.
	AssetCache *novelcache.AssetCache
	// Temperature controls model creativity (0 = deterministic). Default 0.
	Temperature float64
	// Timeout is the per-call deadline. Default 5 minutes.
	Timeout time.Duration
	// Sink is an optional event sink for reasoning/usage events.
	Sink event.Sink
}

// New creates an Orchestrator from the given options.
func New(opts Options) *Orchestrator {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	return &Orchestrator{
		mgr:     sandbox.Manager(),
		caller: &defaultCaller{
			prov:        opts.Provider,
			temperature: opts.Temperature,
			sink:        opts.Sink,
		},
		cache:       opts.AssetCache,
		temperature: opts.Temperature,
		timeout:     opts.Timeout,
	}
}

// --- Task methods ---

// TaskResult carries the outcome of one auxiliary-agent task.
type TaskResult struct {
	Role    string // which role ran the task
	Sandbox string // sandbox ID
	Text    string // full model output
	Cache   string // which cache asset was updated (empty if none)
	Usage   *provider.Usage
	Error   error
}

// BuildWorldview runs the worldbuilding task. It sends a structured prompt
// to the world_builder sandbox and stores the result in the asset cache.
func (o *Orchestrator) BuildWorldview(ctx context.Context, requirements string) TaskResult {
	return o.runTask(ctx, TaskSpec{
		Role:    "world_builder",
		Prompt:  requirements,
		Asset:   novelcache.AssetWorldbuilding,
		Timeout: o.timeout,
	})
}

// DesignCharacters runs the character-design task.
func (o *Orchestrator) DesignCharacters(ctx context.Context, requirements string) TaskResult {
	return o.runTask(ctx, TaskSpec{
		Role:    "character_designer",
		Prompt:  requirements,
		Asset:   novelcache.AssetCharacters,
		Timeout: o.timeout,
	})
}

// BuildOutline runs the outline/plot-structure task.
func (o *Orchestrator) BuildOutline(ctx context.Context, requirements string) TaskResult {
	return o.runTask(ctx, TaskSpec{
		Role:    "outliner",
		Prompt:  requirements,
		Asset:   novelcache.AssetOutline,
		Timeout: o.timeout,
	})
}

// QualityReview runs the quality-review task. The result is NOT cached as an
// asset (it's a diagnostic, not a setting) — the caller should present it
// to the user directly.
func (o *Orchestrator) QualityReview(ctx context.Context, content string) TaskResult {
	return o.runTask(ctx, TaskSpec{
		Role:    "reviewer",
		Prompt:  content,
		Timeout: o.timeout,
		// No asset write — review output is consumable, not reusable
	})
}

// MarketAnalysis runs the planning/market-research task.
func (o *Orchestrator) MarketAnalysis(ctx context.Context, brief string) TaskResult {
	return o.runTask(ctx, TaskSpec{
		Role:    "planner",
		Prompt:  brief,
		Timeout: o.timeout,
		// Market analysis is not a writing asset — caller handles result
	})
}

// RunCustom spawns a task on an arbitrary role. If asset is non-empty,
// the result is written to the asset cache.
func (o *Orchestrator) RunCustom(ctx context.Context, role, prompt string, asset novelcache.AssetType) TaskResult {
	spec := TaskSpec{Role: role, Prompt: prompt, Timeout: o.timeout}
	if asset != "" {
		spec.Asset = asset
	}
	return o.runTask(ctx, spec)
}

// --- Internal execution ---

// TaskSpec describes one auxiliary-agent job.
type TaskSpec struct {
	Role    string              // sandbox role key
	Prompt  string              // the instruction to send
	Asset   novelcache.AssetType // if non-empty, cache the result under this type
	Timeout time.Duration       // per-call deadline; 0 uses the orchestrator default
}

func (o *Orchestrator) runTask(ctx context.Context, spec TaskSpec) TaskResult {
	result := TaskResult{Role: spec.Role}

	// 1. Get (or create) the sandbox for this role.
	prompt := sandbox.RolePrompt(spec.Role)
	if prompt == "" {
		result.Error = fmt.Errorf("orchestrator: unknown role %q", spec.Role)
		return result
	}
	sb := o.mgr.GetOrCreate(spec.Role, prompt)
	result.Sandbox = sb.ID()

	// 2. Push the task instruction into the sandbox.
	sb.PushUser(spec.Prompt)

	// 3. Call the model with only this sandbox's context.
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = o.timeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := provider.Request{
		Messages:    sb.Messages(),
		Temperature: o.temperature,
	}
	text, usage, err := o.caller.Send(callCtx, req)
	result.Text = text
	result.Usage = usage
	if err != nil {
		result.Error = fmt.Errorf("model call for %q: %w", spec.Role, err)
		return result
	}

	// 4. Record the assistant response in the sandbox.
	sb.PushAssistant(text)

	// 5. If this task produces an asset, write it to the cache.
	if spec.Asset != "" {
		o.cache.Set(novelcache.Asset{
			Type:    spec.Asset,
			Content: text,
		})
		result.Cache = string(spec.Asset)
	}

	return result
}

// --- Sandbox management ---

// ResetRole clears a sandbox's conversation history (keeping the system prompt).
func (o *Orchestrator) ResetRole(role string) {
	if sb := o.mgr.Lookup(role); sb != nil {
		sb.Reset()
	}
}

// DestroyRole removes a sandbox entirely. The next task for this role starts fresh.
func (o *Orchestrator) DestroyRole(role string) bool {
	return o.mgr.Destroy(role)
}

// DestroyAll removes every auxiliary sandbox (writer is preserved).
func (o *Orchestrator) DestroyAll() {
	o.mgr.DestroyAll()
}

// SandboxIDs returns a snapshot of managed sandboxes and their IDs.
func (o *Orchestrator) SandboxIDs() map[string]string {
	return o.mgr.List()
}

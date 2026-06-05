// Package tool defines the Tool interface and Registry (mirrors Reasonix pattern).
// Built-in tools self-register via init() → RegisterBuiltin().
package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Tool is the interface every tool must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
	ReadOnly() bool
}

// ---- Global built-in registry ----

var builtinMu sync.Mutex
var builtins = map[string]Tool{}

// RegisterBuiltin registers a compile-time built-in tool. Duplicates panic.
func RegisterBuiltin(t Tool) {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	if _, ok := builtins[t.Name()]; ok {
		panic("tool: duplicate built-in " + t.Name())
	}
	builtins[t.Name()] = t
}

// Builtins returns all registered built-in tools.
func Builtins() map[string]Tool {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	m := make(map[string]Tool, len(builtins))
	for k, v := range builtins {
		m[k] = v
	}
	return m
}

// ---- Runtime Registry ----

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}, order: []string{}}
}

func (r *Registry) Add(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// All returns a snapshot of all tools.
func (r *Registry) All() map[string]Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]Tool, len(r.tools))
	for k, v := range r.tools {
		m[k] = v
	}
	return m
}

// Schemas returns JSON Schemas for all tools, sorted by name.
func (r *Registry) Schemas() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]map[string]any, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  json.RawMessage(t.Schema()),
			},
		})
	}
	return out
}

// ---- Permission Gate ----

// Gate controls whether a tool invocation is permitted.
type Gate interface {
	Check(ctx context.Context, toolName string, readOnly bool) (allow bool, reason string)
}

// AlwaysGate allows everything.
type AlwaysGate struct{}

func (g AlwaysGate) Check(_ context.Context, _ string, _ bool) (bool, string) {
	return true, ""
}

// PlanGate denies non-read-only tools when planMode is true.
type PlanGate struct {
	PlanMode bool
	Fallback Gate
}

func (g PlanGate) Check(ctx context.Context, toolName string, readOnly bool) (bool, string) {
	if g.PlanMode && !readOnly {
		if g.Fallback != nil {
			allow, _ := g.Fallback.Check(ctx, toolName, readOnly)
			if !allow {
				return false, "Plan 模式：写操作已拦截。按 Shift+Tab 切换至 Agent 后重试。"
			}
		}
		return false, "Plan✎ 模式：只读。切换到 Agent 模式执行写操作。"
	}
	if g.Fallback != nil {
		return g.Fallback.Check(ctx, toolName, readOnly)
	}
	return true, ""
}

// --- Helpers ---

func ObjSchema(props map[string]any, required []string) json.RawMessage {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	b, _ := json.Marshal(s)
	return b
}

func Prop(name, typ, desc string) map[string]any {
	return map[string]any{name: map[string]any{"type": typ, "description": desc}}
}

// MergeMerge combines multiple Prop maps.
func MergeProps(props ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, p := range props {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

// ---- Context helpers ----

// RootCtxKey is the context key for the .novelAgent root path.
type rootCtxKey struct{}

// WithRoot stores the root directory in the context.
func WithRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, rootCtxKey{}, root)
}

// RootFrom extracts the root directory from context.
func RootFrom(ctx context.Context) string {
	if v := ctx.Value(rootCtxKey{}); v != nil {
		return v.(string)
	}
	return ""
}

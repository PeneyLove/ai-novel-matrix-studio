package sandbox

import (
	"fmt"
	"sync"
)

// SandboxManager owns the lifecycle of all AgentSandbox instances.
// It is intentionally a singleton (per-process) so every component
// that needs a sandbox finds the same set. Helper agents reuse
// sandboxes of the same role (accumulating context across calls),
// while the main writer sandbox is registered once and never destroyed.
type SandboxManager struct {
	sandboxes map[string]*AgentSandbox
	mu        sync.RWMutex
}

var (
	instance     *SandboxManager
	instanceOnce sync.Once
)

// Manager returns the process-wide SandboxManager singleton.
func Manager() *SandboxManager {
	instanceOnce.Do(func() {
		instance = &SandboxManager{
			sandboxes: make(map[string]*AgentSandbox),
		}
	})
	return instance
}

// RegisterWriter creates (or replaces) the main writer sandbox and returns it.
// The writer sandbox is special: it is the one the user interacts with directly,
// and it is never destroyed by DestroySandbox. Calling RegisterWriter more than
// once replaces the previous writer sandbox.
func (m *SandboxManager) RegisterWriter(systemPrompt string) *AgentSandbox {
	m.mu.Lock()
	defer m.mu.Unlock()
	sb := NewAgentSandbox("writer", systemPrompt)
	m.sandboxes["writer"] = sb
	return sb
}

// RegisterWriterSandbox sets an externally-created sandbox as the main writer.
// This is the preferred path when the existing Agent session needs to be the
// writer sandbox without duplicating history.
func (m *SandboxManager) RegisterWriterSandbox(sb *AgentSandbox) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sandboxes["writer"] = sb
}

// GetOrCreate returns an existing sandbox for role, or creates a new one with
// the given system prompt. Uses double-checked locking so the common path
// (sandbox already exists) is cheap.
func (m *SandboxManager) GetOrCreate(role, systemPrompt string) *AgentSandbox {
	m.mu.RLock()
	sb, ok := m.sandboxes[role]
	m.mu.RUnlock()
	if ok {
		return sb
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if sb, ok := m.sandboxes[role]; ok {
		return sb
	}
	sb = NewAgentSandbox(role, systemPrompt)
	m.sandboxes[role] = sb
	return sb
}

// Writer returns the main writer sandbox, or nil when none is registered.
func (m *SandboxManager) Writer() *AgentSandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sandboxes["writer"]
}

// Lookup returns the sandbox for role, or nil when absent.
func (m *SandboxManager) Lookup(role string) *AgentSandbox {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sandboxes[role]
}

// Destroy removes role's sandbox, releasing its memory. The writer sandbox
// is never destroyed. Returns false when role was not found or is "writer".
func (m *SandboxManager) Destroy(role string) bool {
	if role == "writer" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sandboxes[role]; !ok {
		return false
	}
	delete(m.sandboxes, role)
	return true
}

// DestroyAll removes every sandbox except the writer.
func (m *SandboxManager) DestroyAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for role := range m.sandboxes {
		if role == "writer" {
			continue
		}
		delete(m.sandboxes, role)
	}
}

// List returns a snapshot of the managed roles and their sandbox IDs.
func (m *SandboxManager) List() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.sandboxes))
	for role, sb := range m.sandboxes {
		out[role] = sb.ID()
	}
	return out
}

// String returns a compact debug representation.
func (m *SandboxManager) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("SandboxManager(%d sandboxes)", len(m.sandboxes))
}

// ResetForTesting destroys all sandboxes including the writer, resets the
// singleton, and returns a fresh Manager. Only for use in tests.
func ResetForTesting() *SandboxManager {
	instanceOnce = sync.Once{}
	instance = nil
	m := Manager()
	return m
}

// Package sandbox provides memory-isolated conversation contexts for
// multi-agent collaboration. Each sandbox is an independent "chat window"
// — sandboxes share no memory references; the only cross-sandbox data
// channel is the application-level cache (internal/cache/novel).
//
// Design invariants:
//   - Two sandboxes never share a Message slice backing array.
//   - GetContext returns a deep copy; external mutation can't corrupt history.
//   - All public methods are safe for concurrent use.
package sandbox

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

// AgentSandbox is a fully isolated conversation context for one agent role.
// It is the minimum isolation unit — each sandbox is equivalent to a
// separate chat window with its own message history, system prompt, and
// lifecycle.
type AgentSandbox struct {
	id           string
	roleType     string
	systemPrompt string
	messages     []provider.Message // system prompt at index 0
	mu           sync.RWMutex
}

// NewAgentSandbox creates a sandbox with the given role and system prompt.
// The system prompt is prepended as the first message.
func NewAgentSandbox(roleType, systemPrompt string) *AgentSandbox {
	return &AgentSandbox{
		id:           "sandbox_" + roleType + "_" + randomHex(8),
		roleType:     roleType,
		systemPrompt: systemPrompt,
		messages: []provider.Message{
			{Role: provider.RoleSystem, Content: systemPrompt},
		},
	}
}

// NewAgentSandboxWithSeed is like NewAgentSandbox but accepts a deterministic
// seed suffix for the ID (useful in tests where stable IDs help debugging).
func NewAgentSandboxWithSeed(roleType, systemPrompt, seed string) *AgentSandbox {
	return &AgentSandbox{
		id:           "sandbox_" + roleType + "_" + seed,
		roleType:     roleType,
		systemPrompt: systemPrompt,
		messages: []provider.Message{
			{Role: provider.RoleSystem, Content: systemPrompt},
		},
	}
}

// Push adds a message to this sandbox's history. Only writes to this sandbox;
// other sandboxes are unaffected.
func (s *AgentSandbox) Push(m provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

// PushUser is a convenience wrapper for appending a user message.
func (s *AgentSandbox) PushUser(content string) {
	s.Push(provider.Message{Role: provider.RoleUser, Content: content})
}

// PushAssistant is a convenience wrapper for appending an assistant message.
func (s *AgentSandbox) PushAssistant(content string) {
	s.Push(provider.Message{Role: provider.RoleAssistant, Content: content})
}

// Messages returns a deep copy of the full message history. Callers cannot
// mutate the sandbox's internal state through the returned slice.
func (s *AgentSandbox) Messages() []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]provider.Message(nil), s.messages...)
}

// MessageCount returns the number of messages in the sandbox (including the
// system prompt).
func (s *AgentSandbox) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.messages)
}

// Reset clears all business conversation, restoring only the system prompt.
// The sandbox ID and role are preserved.
func (s *AgentSandbox) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = []provider.Message{
		{Role: provider.RoleSystem, Content: s.systemPrompt},
	}
}

// Role returns the sandbox role identifier (e.g. "writer", "outliner").
func (s *AgentSandbox) Role() string { return s.roleType }

// ID returns the unique sandbox identifier.
func (s *AgentSandbox) ID() string { return s.id }

// SystemPrompt returns the sandbox's system prompt.
func (s *AgentSandbox) SystemPrompt() string { return s.systemPrompt }

// UpdateSystemPrompt replaces the system prompt and rewrites the first message.
// Existing conversation messages are preserved.
func (s *AgentSandbox) UpdateSystemPrompt(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemPrompt = prompt
	if len(s.messages) > 0 && s.messages[0].Role == provider.RoleSystem {
		s.messages[0].Content = prompt
	} else {
		s.messages = append([]provider.Message{
			{Role: provider.RoleSystem, Content: prompt},
		}, s.messages...)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

package sandbox

import (
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

func TestNewAgentSandbox(t *testing.T) {
	sb := NewAgentSandbox("test_role", "You are a test agent.")
	if sb.Role() != "test_role" {
		t.Fatalf("Role() = %q, want %q", sb.Role(), "test_role")
	}
	if sb.ID() == "" {
		t.Fatal("ID() is empty")
	}
	if sb.SystemPrompt() != "You are a test agent." {
		t.Fatalf("SystemPrompt() = %q", sb.SystemPrompt())
	}
	if sb.MessageCount() != 1 {
		t.Fatalf("MessageCount() = %d, want 1", sb.MessageCount())
	}

	msgs := sb.Messages()
	if len(msgs) != 1 || msgs[0].Role != provider.RoleSystem {
		t.Fatalf("first message should be system: %+v", msgs[0])
	}
}

func TestPushMessages(t *testing.T) {
	sb := NewAgentSandbox("t", "sys")
	sb.PushUser("hello")
	sb.PushAssistant("hi there")

	if sb.MessageCount() != 3 {
		t.Fatalf("MessageCount() = %d, want 3", sb.MessageCount())
	}

	msgs := sb.Messages()
	if msgs[1].Role != provider.RoleUser || msgs[1].Content != "hello" {
		t.Fatalf("user message: %+v", msgs[1])
	}
	if msgs[2].Role != provider.RoleAssistant || msgs[2].Content != "hi there" {
		t.Fatalf("assistant message: %+v", msgs[2])
	}
}

func TestMessagesDeepCopy(t *testing.T) {
	sb := NewAgentSandbox("t", "sys")
	sb.PushUser("a")

	msgs1 := sb.Messages()
	msgs2 := sb.Messages()

	// Mutating one copy must not affect the other.
	msgs1[0] = provider.Message{Role: provider.RoleUser, Content: "corrupt"}

	msgs3 := sb.Messages()
	if msgs3[0].Role != provider.RoleSystem {
		t.Fatal("internal state was corrupted through returned slice")
	}
	if msgs2[0].Role != provider.RoleSystem {
		t.Fatal("earlier copy was corrupted through later mutation")
	}
}

func TestReset(t *testing.T) {
	sb := NewAgentSandbox("t", "sys")
	sb.PushUser("a")
	sb.PushAssistant("b")
	sb.Reset()

	if sb.MessageCount() != 1 {
		t.Fatalf("after reset MessageCount() = %d, want 1", sb.MessageCount())
	}
	if sb.Messages()[0].Content != "sys" {
		t.Fatalf("system prompt changed after reset: %q", sb.Messages()[0].Content)
	}
}

func TestUpdateSystemPrompt(t *testing.T) {
	sb := NewAgentSandbox("t", "old")
	sb.PushUser("a")
	sb.UpdateSystemPrompt("new")

	msgs := sb.Messages()
	if msgs[0].Content != "new" {
		t.Fatalf("system prompt not updated: %q", msgs[0].Content)
	}
	if msgs[1].Content != "a" {
		t.Fatalf("user message lost after prompt update: %+v", msgs[1])
	}
	if sb.SystemPrompt() != "new" {
		t.Fatalf("SystemPrompt() stale: %q", sb.SystemPrompt())
	}
}

func TestConcurrency(t *testing.T) {
	sb := NewAgentSandbox("t", "sys")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			sb.PushUser("ping")
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = sb.Messages()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	// Should not panic or deadlock.
	if sb.MessageCount() < 100 {
		t.Fatalf("expected at least 100 messages, got %d", sb.MessageCount())
	}
}

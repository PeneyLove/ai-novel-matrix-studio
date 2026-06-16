package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/agent/sandbox"
	novelcache "github.com/PeneyLove/ai-novel-matrix-studio/internal/cache/novel"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/provider"
)

// fakeCaller implements ModelCaller for testing.
type fakeCaller struct {
	reply string
	usage *provider.Usage
	calls int
}

func (f *fakeCaller) Send(ctx context.Context, req provider.Request) (string, *provider.Usage, error) {
	f.calls++
	// Echo the last user message as a simple test reply.
	last := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == provider.RoleUser {
			last = req.Messages[i].Content
			break
		}
	}
	if f.reply != "" {
		return f.reply, f.usage, nil
	}
	return "reply: " + last, &provider.Usage{PromptTokens: 100, CompletionTokens: 20}, nil
}

func TestOrchestrator_Worldbuilding(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{
		reply: "# 世界观设定\n力量体系：修真",
	}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	result := orch.BuildWorldview(context.Background(), "创建一个修真世界观")
	if result.Error != nil {
		t.Fatalf("BuildWorldview: %v", result.Error)
	}
	if !strings.Contains(result.Text, "修真") {
		t.Fatalf("result does not contain expected content: %q", result.Text)
	}
	if result.Role != "world_builder" {
		t.Fatalf("role = %q, want world_builder", result.Role)
	}
	if caller.calls != 1 {
		t.Fatalf("expected 1 model call, got %d", caller.calls)
	}

	// Asset cache should contain the worldbuilding result.
	asset := cache.Get(novelcache.AssetWorldbuilding)
	if asset == nil {
		t.Fatal("worldbuilding result was not cached")
	}
}

func TestOrchestrator_QualityReview(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{reply: "评分：85/100，伏笔回收率偏低"}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	result := orch.QualityReview(context.Background(), "第5章内容...")
	if result.Error != nil {
		t.Fatalf("QualityReview: %v", result.Error)
	}
	// Review results are NOT cached as assets.
	if cache.Count() != 0 {
		t.Fatal("review results should not be cached as assets")
	}
}

func TestOrchestrator_SandboxReuse(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	// First task: worldbuilding.
	orch.BuildWorldview(context.Background(), "task 1")
	// Second task: same role, should reuse sandbox.
	orch.BuildWorldview(context.Background(), "task 2")

	ids := orch.SandboxIDs()
	if len(ids) != 1 {
		t.Fatalf("expected 1 sandbox, got %d: %v", len(ids), ids)
	}

	// Both calls went to the same sandbox (persistent history).
	sb := sandbox.Manager().Lookup("world_builder")
	if sb == nil {
		t.Fatal("world_builder sandbox should exist")
	}
	if sb.MessageCount() < 4 { // system + user1 + assistant1 + user2
		t.Fatalf("world_builder should have accumulated messages, got %d", sb.MessageCount())
	}
}

func TestOrchestrator_DestroyAll(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	orch.BuildWorldview(context.Background(), "task")
	orch.DesignCharacters(context.Background(), "task")
	if len(orch.SandboxIDs()) != 2 {
		t.Fatalf("expected 2 sandboxes after tasks, got %d", len(orch.SandboxIDs()))
	}

	orch.DestroyAll()
	if len(orch.SandboxIDs()) != 0 {
		t.Fatalf("expected 0 sandboxes after DestroyAll, got %d", len(orch.SandboxIDs()))
	}
}

func TestOrchestrator_UnknownRole(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	result := orch.RunCustom(context.Background(), "nonexistent", "do something", "")
	if result.Error == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestOrchestrator_ResetRole(t *testing.T) {
	sandbox.ResetForTesting()
	cache := novelcache.NewAssetCache()
	caller := &fakeCaller{}

	orch := &Orchestrator{
		mgr:    sandbox.Manager(),
		caller: caller,
		cache:  cache,
	}

	orch.BuildWorldview(context.Background(), "task 1")
	orch.ResetRole("world_builder")

	sb := sandbox.Manager().Lookup("world_builder")
	if sb == nil {
		t.Fatal("world_builder should exist after reset")
	}
	if sb.MessageCount() != 1 {
		t.Fatalf("after reset, MessageCount = %d, want 1 (system only)", sb.MessageCount())
	}
}

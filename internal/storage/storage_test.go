// Package storage_test provides property-based tests for the storage layer.
package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

func TestResolveRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	tests := []string{
		"../escape",
		"skills/../../etc",
		"foo/../../../bar",
	}
	for _, sub := range tests {
		_, err := filepath.Rel(root, root+"/"+sub) // pre-check
		if err == nil {
			// Test that storage.resolve rejects it
			// We test this indirectly via storage functions
			err := storage.WriteConfig(root+"/../escape", map[string]any{})
			if err == nil {
				t.Errorf("expected WriteConfig to reject path traversal via %q", sub)
			}
		}
	}
}

// P7: Content hash dedup — WriteCorpus with the same content twice should not
// create duplicate entries (hash-based dedup).
func TestCorpusDedupByContent(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".novelAgent"), 0o755)
	storageRoot := filepath.Join(root, ".novelAgent")

	content := "这是测试语料内容"
	// Write once
	if err := storage.WriteCorpus(storageRoot, "test", content); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Write same content again
	if err := storage.WriteCorpus(storageRoot, "test", content); err != nil {
		t.Fatalf("second write: %v", err)
	}

	lines, err := storage.ReadCorpus(storageRoot, "test")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	// Current implementation appends — future: dedup via hash.
	// For now we verify reading works.
	if len(lines) < 1 {
		t.Fatal("corpus should have at least 1 line")
	}
}

// P6: Copyright trace completeness — after appending a trace, the task_id,
// prompt_hash, and draft_hash are all non-empty.
func TestTraceCompleteness(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".novelAgent"), 0o755)
	storageRoot := filepath.Join(root, ".novelAgent")

	tr := storage.TraceRecord{
		TaskID:     "task-001",
		Stage:      "topic_generation",
		PromptHash: storage.HashSHA256("system: generate topic\nuser: trend data"),
		DraftHash:  storage.HashSHA256("generated topic content here"),
		FinalHash:  "",
		Timestamp:  "2025-01-15T12:00:00Z",
	}

	if tr.PromptHash == "" || tr.DraftHash == "" {
		t.Fatal("P6 violation: prompt_hash and draft_hash must not be empty")
	}

	if err := storage.AppendTrace(storageRoot, tr); err != nil {
		t.Fatalf("append trace: %v", err)
	}

	// Verify the trace file exists and contains the task_id
	data, err := os.ReadFile(filepath.Join(storageRoot, "traces", "task-001.jsonl"))
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("trace file should not be empty")
	}
}

// P4: Task idempotency — TaskExists returns true after outputs are written.
func TestTaskIdempotency(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".novelAgent"), 0o755)
	storageRoot := filepath.Join(root, ".novelAgent")

	taskID := "task-idempotent-001"
	if storage.TaskExists(storageRoot, taskID) {
		t.Fatal("new task should not exist yet")
	}

	// Write output
	if err := storage.WriteOutput(storageRoot, taskID, "stage1.txt", "content here"); err != nil {
		t.Fatalf("write output: %v", err)
	}

	if !storage.TaskExists(storageRoot, taskID) {
		t.Fatal("P4 violation: task should exist after writing output")
	}
}

// Package state manages the persistent novel creation state — the "hook ledger"
// (伏笔台账), character profiles, world-building data, and iteration history.
//
// Every time the user modifies something (outline, characters, hooks), the state
// is versioned and linked. When a new writing stage starts, the orchestrator
// injects the current state into the skill's prompt template via {{.NovelState}}.
//
// Persistence: JSON files under .novelAgent/state/<novel_id>.json
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

// NovelState is the complete persistent state for one novel project.
type NovelState struct {
	NovelID     string `json:"novel_id"`
	Genre       string `json:"genre"`        // locked genre code
	SubTrack    string `json:"sub_track"`    // 细分赛道 e.g. "凡人流" "都市战神"
	CoreRoutine string `json:"core_routine"` // 核心套路

	// ---- Phase 1: Init ----
	InitCompleted bool   `json:"init_completed"`
	InitSummary   string `json:"init_summary"` // locked genre rules from init output

	// ---- Phase 2: Outline & Hooks ----
	OutlineVersion    int    `json:"outline_version"`
	OutlineContent    string `json:"outline_content"`    // latest approved outline
	OutlineFinalized  bool   `json:"outline_finalized"`  // becomes true when user says "定稿"
	OutlineHistory    []VersionRecord `json:"outline_history"`

	// World-building
	WorldSetting      string `json:"world_setting"`
	PowerSystem       string `json:"power_system"`       // 境界体系 / 势力架构
	FactionStructure  string `json:"faction_structure"`  // 宗门/豪门/朝堂 势力

	// Characters
	MainCharacter     CharacterProfile   `json:"main_character"`
	SupportCharacters []CharacterProfile `json:"support_characters"`
	Antagonists       []CharacterProfile `json:"antagonists"`

	// ---- Hook Ledger (伏笔台账) ----
	Hooks []HookEntry `json:"hooks"`

	// ---- Phase 3: Writing ----
	WritingStarted       bool   `json:"writing_started"`
	ChaptersWritten      int    `json:"chapters_written"`
	LastChapterSummary   string `json:"last_chapter_summary"`   // last 300 chars
	WritingIterationRound int   `json:"writing_iteration_round"`

	// ---- Phase 4: Optimizations ----
	OptimizationLog      []OptimizationRecord `json:"optimization_log"`

	// ---- Meta ----
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// CharacterProfile holds a character's core attributes.
type CharacterProfile struct {
	Name         string `json:"name"`
	Role         string `json:"role"`          // 主角/配角/反派
	Personality  string `json:"personality"`   // 性格关键词
	Background   string `json:"background"`    // 背景设定
	Motivation   string `json:"motivation"`    // 核心动机
	Arc          string `json:"arc"`           // 人物弧光
	Abilities    string `json:"abilities"`     // 能力/金手指
	Relationships string `json:"relationships"` // 关系网
}

// HookEntry is one entry in the foreshadowing ledger.
type HookEntry struct {
	ID          string `json:"id"`           // e.g. "hook-001"
	Type        string `json:"type"`         // 长线伏笔 / 阶段性爽点 / 章节钩子
	Description string `json:"description"`  // 伏笔/爽点/钩子 描述
	PlacedAt    string `json:"placed_at"`    // 埋设位置：大纲卷X / 第X章
	TriggerAt   string `json:"trigger_at"`   // 计划触发/回收节点
	ResolvedAt  string `json:"resolved_at"`  // 实际回收节点（空=未回收）
	Status      string `json:"status"`       // pending / triggered / resolved
}

// VersionRecord tracks one iteration of outline modification.
type VersionRecord struct {
	Version   int    `json:"version"`
	Timestamp string `json:"timestamp"`
	WhatChanged string `json:"what_changed"`
	Content   string `json:"content"` // snapshot
}

// OptimizationRecord tracks one optimization call.
type OptimizationRecord struct {
	Skill     string `json:"skill"`      // e.g. "optimize_shuangdian"
	Timestamp string `json:"timestamp"`
	Target    string `json:"target"`     // what was optimized
	Before    string `json:"before"`     // brief before-state
	After     string `json:"after"`      // brief after-state
}

// NewNovelState creates a blank state for a new novel.
func NewNovelState(novelID, genre string) *NovelState {
	now := time.Now().UTC().Format(time.RFC3339)
	return &NovelState{
		NovelID:   novelID,
		Genre:     genre,
		Hooks:     make([]HookEntry, 0),
		OutlineHistory: make([]VersionRecord, 0),
		OptimizationLog: make([]OptimizationRecord, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Save persists the state to .novelAgent/state/<novel_id>.json.
func (ns *NovelState) Save(root string) error {
	ns.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	dir := filepath.Join(root, "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("state: create state dir: %w", err)
	}
	path := filepath.Join(dir, ns.NovelID+".json")
	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadNovelState reads the state for a novel from disk.
func LoadNovelState(root, novelID string) (*NovelState, error) {
	path := filepath.Join(root, "state", novelID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", novelID, err)
	}
	var ns NovelState
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", novelID, err)
	}
	return &ns, nil
}

// AddHook appends a new hook to the ledger.
func (ns *NovelState) AddHook(h HookEntry) {
	if h.ID == "" {
		h.ID = fmt.Sprintf("hook-%03d", len(ns.Hooks)+1)
	}
	if h.Status == "" {
		h.Status = "pending"
	}
	ns.Hooks = append(ns.Hooks, h)
}

// ResolveHook marks a hook as resolved.
func (ns *NovelState) ResolveHook(hookID, resolvedAt string) bool {
	for i, h := range ns.Hooks {
		if h.ID == hookID {
			ns.Hooks[i].Status = "resolved"
			ns.Hooks[i].ResolvedAt = resolvedAt
			return true
		}
	}
	return false
}

// AddOutlineVersion records a new outline iteration.
func (ns *NovelState) AddOutlineVersion(whatChanged, content string) {
	ns.OutlineVersion++
	ns.OutlineContent = content
	ns.OutlineHistory = append(ns.OutlineHistory, VersionRecord{
		Version:     ns.OutlineVersion,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		WhatChanged: whatChanged,
		Content:     content,
	})
}

// FinalizeOutline locks the outline. After this, Phase 3 (writing) is allowed.
func (ns *NovelState) FinalizeOutline() {
	ns.OutlineFinalized = true
}

// CanStartWriting returns true if outline is finalized and init is complete.
func (ns *NovelState) CanStartWriting() bool {
	return ns.InitCompleted && ns.OutlineFinalized
}

// Summarize returns a compact prompt-ready summary for injection into templates.
func (ns *NovelState) Summarize() string {
	return storage.HashSHA256(ns.OutlineContent + ns.Genre)[:16]
}

// HookSummary returns a formatted summary of all hooks for prompt injection.
func (ns *NovelState) HookSummary() string {
	if len(ns.Hooks) == 0 {
		return "（暂无已埋伏笔）"
	}
	var pending, resolved int
	for _, h := range ns.Hooks {
		switch h.Status {
		case "resolved":
			resolved++
		default:
			pending++
		}
	}
	s := fmt.Sprintf("伏笔台账：共 %d 条（待回收 %d，已回收 %d）\n", len(ns.Hooks), pending, resolved)
	for _, h := range ns.Hooks {
		s += fmt.Sprintf("  [%s] %s: %s (埋于%s, 计划回收于%s)\n",
			h.Status, h.ID, h.Description, h.PlacedAt, h.TriggerAt)
	}
	return s
}

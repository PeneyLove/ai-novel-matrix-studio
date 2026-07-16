// Package timeline provides the Timeline Synthesis Agent — the central
// orchestrator that collects independent character-agent reactions, sorts
// them by web-novel rhythm rules, resolves conflicts, and produces both
// chapter output and graph update instructions.
//
// Key responsibilities (from the design doc):
//   1. Collect reaction candidates from all involved character agents.
//   2. Sort/integrate by web-novel rhythm: 铺垫 → 冲突升级 → 高潮/爽点 → 悬念收尾.
//   3. Filter/coordinate reactions into a self-consistent final version.
//      When conflicts exist, prioritise the protagonist's story line while
//      not breaking supporting characters' personality red-lines.
//   4. Output: chapter outline (or full text) + graph UpdateBatch.
//
// Design invariant: the Timeline Agent is the ONLY writer to the Story Bible
// Graph. Character agents are read-only consumers of snapshots. This
// single-writer model prevents consistency drift.
package timeline

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
)

// ─── Rhythm Phase ───────────────────────────────────────────────────────────

// RhythmPhase represents a beat in the web-novel rhythm structure.
type RhythmPhase string

const (
	PhaseSetup    RhythmPhase = "铺垫"   // establish scene, character positions
	PhaseConflict RhythmPhase = "冲突升级" // escalate tension, introduce obstacles
	PhaseClimax   RhythmPhase = "高潮/爽点" // peak moment, payoff, face-slapping
	PhaseSuspense RhythmPhase = "悬念收尾"  // hook for next chapter, unresolved thread
)

// WebNovelRhythm is the standard 4-beat structure.
var WebNovelRhythm = []RhythmPhase{PhaseSetup, PhaseConflict, PhaseClimax, PhaseSuspense}

// ─── Synthesis Input ────────────────────────────────────────────────────────

// SynthesisInput is the material the timeline agent works with.
type SynthesisInput struct {
	ChapterNum int                              // which chapter is being written
	Reactions  []characteragent.Reaction         // all character reactions to the trigger event
	Profiles   map[string]*characteragent.CharacterProfile // character ID → profile
	ProtagonistID string                         // which character is the protagonist
	GraphSnap  *storybible.Snapshot              // current world state snapshot
	StyleGuide string                            // author style preferences
}

// ─── Synthesis Output ───────────────────────────────────────────────────────

// SynthesisOutput is the complete output of one timeline synthesis run.
type SynthesisOutput struct {
	ChapterNum  int                            // chapter number
	Outline     string                         // chapter outline / scene breakdown
	FullText    string                         // full chapter text (if generated)
	Narrative   []SceneBeat                    // scene-by-scene beats
	GraphBatch  storybible.UpdateBatch         // graph mutations to apply
	ConflictLog []ConflictResolution           // how conflicts were resolved
	Metadata    SynthesisMeta                  // runtime metadata
}

// SceneBeat is one narrative beat in the chapter flow.
type SceneBeat struct {
	Phase        RhythmPhase `json:"phase"`         // which rhythm phase
	Description  string      `json:"description"`   // what happens in this beat
	POVCharacter string      `json:"pov_character"` // whose perspective (node ID)
	Actions      []string    `json:"actions"`       // specific actions taken
	Emotion      string      `json:"emotion"`       // dominant emotion of the beat
}

// ConflictResolution describes how a conflict between character reactions was resolved.
type ConflictResolution struct {
	Between       []string `json:"between"`        // conflicting character names
	ConflictDesc  string   `json:"conflict_desc"`  // what the conflict is
	Resolution    string   `json:"resolution"`     // how it was resolved
	PrioritisedID string   `json:"prioritised_id"` // which character's action prevailed
}

// SynthesisMeta holds runtime metadata about the synthesis run.
type SynthesisMeta struct {
	NumReactions  int           `json:"num_reactions"`
	NumConflicts  int           `json:"num_conflicts"`
	NumGraphMutations int       `json:"num_graph_mutations"`
	Duration      time.Duration `json:"duration"`
}

// ─── Synthesis Engine ───────────────────────────────────────────────────────

// Engine is the timeline synthesis engine. It takes raw character reactions
// and produces structured narrative output + graph update instructions.
// The Engine itself is deterministic Go logic; the LLM-powered integration
// happens at the call-site (e.g. in the tool layer).
type Engine struct {
	protagonistID string
	graph         *storybible.Graph
}

// NewEngine creates a timeline synthesis engine.
func NewEngine(graph *storybible.Graph, protagonistID string) *Engine {
	return &Engine{
		protagonistID: protagonistID,
		graph:         graph,
	}
}

// Synthesize processes character reactions into a structured scene plan
// and a graph update batch. This is the deterministic core; the LLM
// integration layer can call this after parsing model output.
func (e *Engine) Synthesize(input SynthesisInput) *SynthesisOutput {
	start := time.Now()

	out := &SynthesisOutput{
		ChapterNum: input.ChapterNum,
		Metadata: SynthesisMeta{
			NumReactions: len(input.Reactions),
		},
	}

	// Phase 1: Resolve conflicts.
	conflicts := e.detectConflicts(input.Reactions, input.Profiles)
	resolved := e.resolveConflicts(conflicts, input.ProtagonistID)
	out.ConflictLog = resolved
	out.Metadata.NumConflicts = len(conflicts)

	// Phase 2: Build scene beats around the 4-phase rhythm.
	out.Narrative = e.buildSceneBeats(input, resolved)

	// Phase 3: Generate graph update instructions from relationship changes.
	out.GraphBatch = e.buildGraphBatch(input)

	out.Metadata.NumGraphMutations = len(out.GraphBatch.Instructions)
	out.Metadata.Duration = time.Since(start)

	return out
}

// ─── Conflict Detection & Resolution ────────────────────────────────────────

// detectConflicts finds pairs of reactions that are logically incompatible.
// Two actions conflict when they target the same outcome with opposite intent:
// e.g., character A wants to attack B, character C wants to protect B.
func (e *Engine) detectConflicts(reactions []characteragent.Reaction, profiles map[string]*characteragent.CharacterProfile) []ConflictResolution {
	var conflicts []ConflictResolution

	// Simple heuristic: two high-priority actions that are semantically
	// opposed (attack vs protect, reveal vs conceal, etc.) on the same target.
	type actionIntent struct {
		charID   string
		charName string
		target   string // target character name from RelChange
		intent   string // simplified: attack/protect/leave/stay/reveal/conceal
	}

	var intents []actionIntent
	for _, r := range reactions {
		name := r.CharacterName
		for _, a := range r.Actions {
			if a.Priority != "高" {
				continue
			}
			intent := classifyIntent(a.Action)
			// Find target from relationship changes
			target := ""
			for _, rc := range r.RelationshipChanges {
				target = rc.TargetName
			}
			intents = append(intents, actionIntent{charID: r.CharacterID, charName: name, target: target, intent: intent})
		}
	}

	// Pairwise comparison
	for i := 0; i < len(intents); i++ {
		for j := i + 1; j < len(intents); j++ {
			if areOpposed(intents[i].intent, intents[j].intent) {
				conflicts = append(conflicts, ConflictResolution{
					Between:      []string{intents[i].charName, intents[j].charName},
					ConflictDesc: fmt.Sprintf("%s倾向%s，%s倾向%s", intents[i].charName, intents[i].intent, intents[j].charName, intents[j].intent),
				})
			}
		}
	}

	return conflicts
}

// classifyIntent maps an action description to a simplified intent category.
func classifyIntent(action string) string {
	action = strings.ToLower(action)
	switch {
	case strings.Contains(action, "攻击") || strings.Contains(action, "杀") || strings.Contains(action, "出手") || strings.Contains(action, "偷袭"):
		return "攻击"
	case strings.Contains(action, "保护") || strings.Contains(action, "护住") || strings.Contains(action, "掩护") || strings.Contains(action, "救"):
		return "保护"
	case strings.Contains(action, "离开") || strings.Contains(action, "退") || strings.Contains(action, "逃"):
		return "离开"
	case strings.Contains(action, "揭示") || strings.Contains(action, "暴露") || strings.Contains(action, "说出") || strings.Contains(action, "宣布"):
		return "揭示"
	case strings.Contains(action, "隐藏") || strings.Contains(action, "隐瞒") || strings.Contains(action, "遮掩"):
		return "隐藏"
	case strings.Contains(action, "劝说") || strings.Contains(action, "谈判") || strings.Contains(action, "交涉"):
		return "劝说"
	default:
		return "其他"
	}
}

// areOpposed checks if two intents are logically opposed.
func areOpposed(a, b string) bool {
	opposites := map[string]string{
		"攻击": "保护",
		"保护": "攻击",
		"揭示": "隐藏",
		"隐藏": "揭示",
		"离开": "留下",
		"留下": "离开",
	}
	if opp, ok := opposites[a]; ok && opp == b {
		return true
	}
	return false
}

// resolveConflicts resolves detected conflicts. Rule: protagonist wins unless
// it would violate a supporting character's red-line (personality bottom line).
// If protagonist's action would violate a red-line, flag it and defer to
// supporting character's action.
func (e *Engine) resolveConflicts(conflicts []ConflictResolution, protagonistID string) []ConflictResolution {
	for i := range conflicts {
		c := &conflicts[i]

		// Default: protagonist wins.
		// In a full implementation, we'd check red-lines here.
		// For now, prioritise the first name (which is sorted to put protagonist first).
		c.PrioritisedID = protagonistID
		if len(c.Between) > 0 {
			// Simple heuristic: if one of the names is the protagonist, they win
			// TODO: check profiles for red-line violation
			c.Resolution = fmt.Sprintf("优先满足主角剧情线：%s的行动方案被采纳", c.Between[0])
		}
	}
	return conflicts
}

// ─── Scene Beat Builder ─────────────────────────────────────────────────────

// buildSceneBeats arranges character actions into the 4-phase rhythm structure.
func (e *Engine) buildSceneBeats(input SynthesisInput, conflicts []ConflictResolution) []SceneBeat {
	var beats []SceneBeat

	var actions []scorableAction
	for _, r := range input.Reactions {
		for _, a := range r.Actions {
			p := 1
			switch a.Priority {
			case "高":
				p = 3
			case "中":
				p = 2
			}
			actions = append(actions, scorableAction{
				charName: r.CharacterName,
				charID:   r.CharacterID,
				action:   a.Action,
				priority: p,
				emotion:  extractEmotion(r.InternalThought),
			})
		}
	}

	// Sort: protagonist first, then by priority descending.
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].charID == e.protagonistID && actions[j].charID != e.protagonistID {
			return true
		}
		if actions[j].charID == e.protagonistID && actions[i].charID != e.protagonistID {
			return false
		}
		return actions[i].priority > actions[j].priority
	})

	// Distribute into phases
	phaseAssignment := assignPhases(actions, len(WebNovelRhythm))
	for _, pa := range phaseAssignment {
		beats = append(beats, SceneBeat{
			Phase:        WebNovelRhythm[pa.phaseIdx],
			Description:  pa.description,
			POVCharacter: pa.charName,
			Actions:      pa.actions,
			Emotion:      pa.emotion,
		})
	}

	return beats
}

// scorableAction is an internal type used for sorting actions by priority.
type scorableAction struct {
	charName string
	charID   string
	action   string
	priority int // 高=3, 中=2, 低=1
	emotion  string
}

// phaseActions is an internal type for phase assignment.
type phaseActions struct {
	phaseIdx    int
	description string
	charName    string
	actions     []string
	emotion     string
}

func assignPhases(actions []scorableAction, numPhases int) []phaseActions {
	if len(actions) == 0 {
		return nil
	}

	// Distribute actions across phases
	result := make([]phaseActions, numPhases)
	for i := range result {
		result[i].phaseIdx = i
	}

	middleStart := 0
	if len(actions) < numPhases {
		// Fewer actions than phases — fill what we can
		for i, a := range actions {
			if i < numPhases {
				result[i].charName = a.charName
				result[i].actions = []string{a.action}
				result[i].emotion = a.emotion
			}
		}
		// Fill empty phases with descriptive placeholders
		switch {
		case result[0].charName == "":
			result[0].description = "场景铺垫：建立当前局势与角色位置"
		case result[1].charName == "":
			result[1].description = "冲突升级：引入意外因素或阻力"
		case result[2].charName == "":
			result[2].description = "高潮/爽点：关键转折或主角高光时刻"
		case result[3].charName == "":
			result[3].description = "悬念收尾：埋下下章钩子"
		}
		return result
	}

	// Phase 0 (setup): first 1-2 actions
	setupEnd := min(2, len(actions))
	if setupEnd > 0 {
		as := actions[:setupEnd]
		result[0].charName = as[0].charName
		result[0].emotion = as[0].emotion
		for _, a := range as {
			result[0].actions = append(result[0].actions, a.action)
		}
		middleStart = setupEnd
	}

	// Phase 3 (suspense): last action
	if len(actions) > middleStart {
		last := actions[len(actions)-1]
		result[3].charName = last.charName
		result[3].actions = []string{last.action}
		result[3].emotion = last.emotion
	}

	// Phases 1-2: middle actions
	middle := actions[middleStart : len(actions)-1]
	if len(middle) > 0 {
		mid := len(middle) / 2
		// Phase 1 (conflict): first half of middle
		cActions := middle[:mid]
		if len(cActions) > 0 {
			result[1].charName = cActions[0].charName
			result[1].emotion = cActions[0].emotion
			for _, a := range cActions {
				result[1].actions = append(result[1].actions, a.action)
			}
		}
		// Phase 2 (climax): second half of middle
		clActions := middle[mid:]
		if len(clActions) > 0 {
			result[2].charName = clActions[0].charName
			result[2].emotion = clActions[0].emotion
			for _, a := range clActions {
				result[2].actions = append(result[2].actions, a.action)
			}
		}
	}

	return result
}

// ─── Graph Batch Builder ────────────────────────────────────────────────────

// buildGraphBatch produces UpdateInstructions from all character relationship changes.
func (e *Engine) buildGraphBatch(input SynthesisInput) storybible.UpdateBatch {
	batch := storybible.UpdateBatch{
		Chapter: input.ChapterNum,
	}

	// Collect all relationship changes from reactions
	for _, r := range input.Reactions {
		for _, rc := range r.RelationshipChanges {
			// Determine target node ID from name
			targetID := resolveCharacterID(rc.TargetName, input.Profiles)

			// Map relationship change to edge operation
			relKind := mapRelChangeToKind(rc.Change)

			inst := storybible.UpdateInstruction{
				Op:      storybible.OpUpsertEdge,
				FromID:  r.CharacterID,
				ToID:    targetID,
				RelKind: relKind,
				Props: map[string]interface{}{
					"reason":  rc.Reason,
					"chapter": fmt.Sprintf("%d", input.ChapterNum),
				},
				Reason:  rc.Reason,
				Chapter: input.ChapterNum,
			}
			batch.Instructions = append(batch.Instructions, inst)
		}
	}

	return batch
}

// mapRelChangeToKind maps a Chinese relationship change description to a RelKind.
func mapRelChangeToKind(change string) storybible.RelKind {
	change = strings.ToLower(change)
	switch {
	case strings.Contains(change, "敌") || strings.Contains(change, "仇") || strings.Contains(change, "背叛"):
		return storybible.RelEnemy
	case strings.Contains(change, "紧密") || strings.Contains(change, "信任") || strings.Contains(change, "盟"):
		return storybible.RelAlly
	case strings.Contains(change, "师") || strings.Contains(change, "拜"):
		return storybible.RelMentor
	case strings.Contains(change, "爱") || strings.Contains(change, "恋") || strings.Contains(change, "情"):
		return storybible.RelLover
	case strings.Contains(change, "竞") || strings.Contains(change, "对"):
		return storybible.RelRival
	default:
		return storybible.RelCustom
	}
}

// resolveCharacterID finds a character's node ID from their name.
func resolveCharacterID(name string, profiles map[string]*characteragent.CharacterProfile) string {
	for id, p := range profiles {
		if p.Name == name {
			return id
		}
	}
	// Fallback: use name as ID (caller should ensure mapping exists)
	return name
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// extractEmotion extracts a dominant emotion keyword from internal thought text.
func extractEmotion(thought string) string {
	emotions := []string{"愤怒", "恐惧", "冷静", "兴奋", "悲伤", "疑惑", "坚定", "犹豫", "惊喜", "警惕", "决然", "挣扎"}
	for _, em := range emotions {
		if strings.Contains(thought, em) {
			return em
		}
	}
	return "平静"
}

// ─── LLM Prompt Builder ─────────────────────────────────────────────────────

// BuildLLMPrompt constructs the prompt for the LLM-powered synthesis step.
// This is the instruction sent to the Timeline Synthesis Agent model.
func BuildLLMPrompt(input SynthesisInput, reactions []characteragent.Reaction) string {
	var b strings.Builder

	b.WriteString("【角色锁定】你是网文时间线合成Agent。\n")
	b.WriteString("你的任务是将多个角色的独立反应整合为自洽的叙事场景。\n\n")

	b.WriteString("## 网文节奏规则\n")
	b.WriteString("1. 铺垫：建立场景，明确角色位置与当前局势\n")
	b.WriteString("2. 冲突升级：引入意外因素或阻力，拉高张力\n")
	b.WriteString("3. 高潮/爽点：关键转折或主角高光时刻\n")
	b.WriteString("4. 悬念收尾：为下一章埋下钩子\n\n")

	b.WriteString("## 冲突解决规则\n")
	b.WriteString("- 当多个角色的行动发生冲突时，优先满足主角剧情线\n")
	b.WriteString("- 但绝不违背配角的性格底线（绝不会做的事）\n")
	b.WriteString("- 被'优先级覆盖'的角色，其行动仍应以'意图受阻'的方式呈现在叙事中\n\n")

	b.WriteString(fmt.Sprintf("## 本章信息\n- 章节: 第%d章\n- 主角: %s\n\n", input.ChapterNum, input.ProtagonistID))

	b.WriteString("## 各角色反应\n\n")
	for _, r := range reactions {
		b.WriteString(r.Render())
		b.WriteString("\n")
	}

	b.WriteString("\n## 输出要求\n")
	b.WriteString("严格按以下JSON格式输出：\n\n")
	b.WriteString("```json\n")
	b.WriteString(`{
  "outline": "本章大纲（200字内）",
  "scene_beats": [
    {"phase": "铺垫", "description": "...", "pov": "角色名", "actions": ["行动1", "行动2"], "emotion": "情绪"},
    {"phase": "冲突升级", "description": "...", "pov": "角色名", "actions": ["行动1"], "emotion": "情绪"},
    {"phase": "高潮/爽点", "description": "...", "pov": "角色名", "actions": ["行动1"], "emotion": "情绪"},
    {"phase": "悬念收尾", "description": "...", "pov": "角色名", "actions": ["行动1"], "emotion": "情绪"}
  ],
  "graph_updates": [
    {"op": "set_prop", "node_id": "节点ID", "props": {"属性": "新值"}, "reason": "原因"},
    {"op": "upsert_edge", "from_id": "角色A_ID", "to_id": "角色B_ID", "rel_kind": "关系类型", "reason": "原因"}
  ],
  "conflict_resolutions": [
    {"between": ["角色A", "角色B"], "resolution": "解决方案", "prioritised": "角色A"}
  ]
}
`)
	b.WriteString("```\n")

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
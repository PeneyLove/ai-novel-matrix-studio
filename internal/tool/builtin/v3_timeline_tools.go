package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/timeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(characterReact{})
	tool.RegisterBuiltin(timelineSynthesize{})
}

// ─── character_react ────────────────────────────────────────────────────────

type characterReact struct{}

type characterReactArgs struct {
	GraphJSON    string                       `json:"graph_json"`    // story-bible.json 内容
	EventType    string                       `json:"event_type"`    // 触发事件类型
	EventDesc    string                       `json:"event_desc"`    // 触发事件描述
	CharacterIDs []string                     `json:"character_ids"` // 参与反应的节点ID
	Location     string                       `json:"location"`      // 事件地点（可选）
	ChapterNum   int                          `json:"chapter_num"`   // 章节号
	Memories     map[string]characterReactMem `json:"memories"`      // 角色记忆摘要（可选）
}

type characterReactMem struct {
	Events       []string `json:"events"`
	ChapterRange string   `json:"chapter_range"`
}

func (c characterReact) Name() string { return "character_react" }

func (c characterReact) Description() string {
	return "运行角色人格Agent：为指定角色生成独立的人格一致反应。每个角色基于自己的性格约束、图谱快照和记忆摘要，输出内心判断+行动倾向+关系变化。角色Agent之间互不感知彼此输出，只感知图谱中的客观事实。"
}

func (c characterReact) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"graph_json": {"type": "string", "description": "story-bible.json 的完整JSON内容"},
			"event_type": {"type": "string", "description": "触发事件类型：战斗/对话/发现/背叛/突袭/交易/..."},
			"event_desc": {"type": "string", "description": "触发事件描述（一句话概述发生了什么）"},
			"character_ids": {"type": "array", "items": {"type": "string"}, "description": "需要生成反应的角色节点ID列表"},
			"location": {"type": "string", "description": "事件发生地点（可选）"},
			"chapter_num": {"type": "integer", "description": "当前章节号"},
			"memories": {"type": "object", "description": "角色ID到记忆摘要的映射（可选）。每个记忆包含 events(最近事件列表) 和 chapter_range(如'第12-14章')"}
		},
		"required": ["graph_json", "event_type", "event_desc", "character_ids", "chapter_num"]
	}`)
}

func (c characterReact) ReadOnly() bool { return true }

func (c characterReact) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p characterReactArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("character_react: invalid args: %w", err)
	}

	// Parse graph
	g := storybible.NewGraph()
	if err := json.Unmarshal([]byte(p.GraphJSON), g); err != nil {
		return "", fmt.Errorf("character_react: invalid graph JSON: %w", err)
	}

	// Build snapshot for all involved characters
	snap := g.SnapshotForCharacters(p.CharacterIDs)

	// Build profiles from graph nodes
	profiles := make(map[string]*characteragent.CharacterProfile)
	for _, id := range p.CharacterIDs {
		node := g.GetNode(id)
		if node == nil {
			continue
		}
		profile := characteragent.DeriveProfileFromNode(node)
		if profile != nil {
			profiles[id] = profile
		}
	}

	// Build memories
	memories := make(map[string]*characteragent.MemorySummary)
	for id, mem := range p.Memories {
		memories[id] = &characteragent.MemorySummary{
			CharacterID:  id,
			RecentEvents: mem.Events,
			ChapterRange: mem.ChapterRange,
		}
	}

	// Build batch spec
	batchSpec := &characteragent.BatchSpec{
		Profiles:  profileMapToSlice(profiles),
		Memories:  memories,
		GraphSnap: snap,
		Event: characteragent.TriggerEvent{
			EventType:    p.EventType,
			Description:  p.EventDesc,
			Location:     p.Location,
			ChapterNum:   p.ChapterNum,
		},
	}

	// Build all prompts
	prompts := batchSpec.BuildAllPrompts()

	// Since we can't call LLM from Go directly, return the prompts for the model to process.
	// The model should then interpret the prompts and return structured reactions.
	result := fmt.Sprintf("已为 %d 个角色生成反应提示词。请将以下每个角色的提示词发送给LLM，获取YAML格式的反应输出。\n\n", len(prompts))
	result += fmt.Sprintf("图谱快照：%d 节点，%d 边\n", len(snap.Nodes), len(snap.Edges))
	result += fmt.Sprintf("触发事件：%s - %s（第%d章）\n\n", p.EventType, p.EventDesc, p.ChapterNum)

	for id, prompt := range prompts {
		profile := profiles[id]
		name := id
		if profile != nil {
			name = profile.Name
		}
		result += fmt.Sprintf("--- %s 的提示词 ---\n", name)
		result += prompt
		result += "\n\n"
	}

	result += "提示：每个角色返回的标准YAML格式为：\n"
	result += "```yaml\n内心反应: |\n  (判断/情绪/推理)\n行动倾向:\n  - 行动: (描述)\n    优先级: 高|中|低\n    触发条件: (条件)\n关系变化:\n  - 目标角色: (角色名)\n    变化: (关系如何改变)\n    原因: (原因)\n```\n"

	return result, nil
}

func profileMapToSlice(m map[string]*characteragent.CharacterProfile) []*characteragent.CharacterProfile {
	var out []*characteragent.CharacterProfile
	for _, p := range m {
		out = append(out, p)
	}
	return out
}

// ─── timeline_synthesize ───────────────────────────────────────────────────

type timelineSynthesize struct{}

type timelineSynthesizeArgs struct {
	GraphJSON     string                `json:"graph_json"`      // story-bible.json 内容
	ChapterNum    int                   `json:"chapter_num"`     // 章节号
	ProtagonistID string                `json:"protagonist_id"`  // 主角节点ID
	Reactions     []timelineReactInput  `json:"reactions"`       // 角色反应列表
	StyleGuide    string                `json:"style_guide"`     // 风格指导（可选）
}

type timelineReactInput struct {
	CharacterID   string                               `json:"character_id"`
	CharacterName string                               `json:"character_name"`
	InternalThought string                             `json:"internal_thought"`
	Actions        []timelineActionInput               `json:"actions"`
	RelChanges     []timelineRelChangeInput             `json:"rel_changes"`
}

type timelineActionInput struct {
	Action      string `json:"action"`
	Priority    string `json:"priority"`
	TriggerCond string `json:"trigger_cond"`
}

type timelineRelChangeInput struct {
	TargetName string `json:"target_name"`
	Change     string `json:"change"`
	Reason     string `json:"reason"`
}

func (t timelineSynthesize) Name() string { return "timeline_synthesize" }

func (t timelineSynthesize) Description() string {
	return "运行时间线合成Agent：收集所有角色Agent的反应输出，按网文节奏规则（铺垫→冲突升级→高潮/爽点→悬念收尾）整合排序，解决冲突，输出场景节拍+图谱更新指令。"
}

func (t timelineSynthesize) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"graph_json": {"type": "string", "description": "story-bible.json 的完整JSON内容"},
			"chapter_num": {"type": "integer", "description": "当前章节号"},
			"protagonist_id": {"type": "string", "description": "主角的角色节点ID"},
			"reactions": {
				"type": "array",
				"description": "各角色的反应输出（从 character_react 的结果中解析YAML获得）",
				"items": {
					"type": "object",
					"properties": {
						"character_id": {"type": "string"},
						"character_name": {"type": "string"},
						"internal_thought": {"type": "string", "description": "内心反应"},
						"actions": {
							"type": "array",
							"items": {
								"type": "object",
								"properties": {
									"action": {"type": "string"},
									"priority": {"type": "string", "enum": ["高", "中", "低"]},
									"trigger_cond": {"type": "string"}
								},
								"required": ["action", "priority"]
							}
						},
						"rel_changes": {
							"type": "array",
							"items": {
								"type": "object",
								"properties": {
									"target_name": {"type": "string"},
									"change": {"type": "string"},
									"reason": {"type": "string"}
								},
								"required": ["target_name", "change", "reason"]
							}
						}
					},
					"required": ["character_id", "character_name"]
				}
			},
			"style_guide": {"type": "string", "description": "风格指导（可选）：如'节奏快、对话简洁、每段不超过5行'"}
		},
		"required": ["graph_json", "chapter_num", "protagonist_id", "reactions"]
	}`)
}

func (t timelineSynthesize) ReadOnly() bool { return true }

func (t timelineSynthesize) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p timelineSynthesizeArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("timeline_synthesize: invalid args: %w", err)
	}

	// Parse graph for profile derivation
	g := storybible.NewGraph()
	if err := json.Unmarshal([]byte(p.GraphJSON), g); err != nil {
		return "", fmt.Errorf("timeline_synthesize: invalid graph JSON: %w", err)
	}

	// Build profiles from all character nodes in graph
	profiles := make(map[string]*characteragent.CharacterProfile)
	for _, node := range g.NodesByKind(storybible.KindCharacter) {
		profile := characteragent.DeriveProfileFromNode(node)
		if profile != nil {
			profiles[node.ID] = profile
		}
	}

	// Convert input reactions to characteragent.Reaction
	var reactions []characteragent.Reaction
	for _, r := range p.Reactions {
		var actions []characteragent.ActionCandidate
		for _, a := range r.Actions {
			actions = append(actions, characteragent.ActionCandidate{
				Action:      a.Action,
				Priority:    a.Priority,
				TriggerCond: a.TriggerCond,
			})
		}
		var relChanges []characteragent.RelChange
		for _, rc := range r.RelChanges {
			relChanges = append(relChanges, characteragent.RelChange{
				TargetName: rc.TargetName,
				Change:     rc.Change,
				Reason:     rc.Reason,
			})
		}
		reactions = append(reactions, characteragent.Reaction{
			CharacterID:        r.CharacterID,
			CharacterName:      r.CharacterName,
			InternalThought:    r.InternalThought,
			Actions:            actions,
			RelationshipChanges: relChanges,
		})
	}

	// Build synthesis input
	input := timeline.SynthesisInput{
		ChapterNum:    p.ChapterNum,
		Reactions:     reactions,
		Profiles:      profiles,
		ProtagonistID: p.ProtagonistID,
		StyleGuide:    p.StyleGuide,
	}

	// Run deterministic synthesis
	engine := timeline.NewEngine(g, p.ProtagonistID)
	out := engine.Synthesize(input)

	// Build human-readable output
	result := fmt.Sprintf("═ 时间线合成报告 · 第%d章 ═\n\n", p.ChapterNum)

	result += fmt.Sprintf("## 汇总\n")
	result += fmt.Sprintf("- 角色反应数: %d\n", out.Metadata.NumReactions)
	result += fmt.Sprintf("- 冲突检测: %d 项\n", out.Metadata.NumConflicts)
	result += fmt.Sprintf("- 图谱变更: %d 条\n", out.Metadata.NumGraphMutations)
	result += fmt.Sprintf("- 处理耗时: %v\n\n", out.Metadata.Duration)

	// Scene beats
	result += "## 场景节拍\n\n"
	for _, beat := range out.Narrative {
		result += fmt.Sprintf("### 【%s】\n", beat.Phase)
		result += fmt.Sprintf("- POV: %s\n", beat.POVCharacter)
		result += fmt.Sprintf("- 描述: %s\n", beat.Description)
		result += fmt.Sprintf("- 动作: %v\n", beat.Actions)
		result += fmt.Sprintf("- 情绪: %s\n\n", beat.Emotion)
	}

	// Conflict resolutions
	if len(out.ConflictLog) > 0 {
		result += "## 冲突解决\n\n"
		for _, cr := range out.ConflictLog {
			result += fmt.Sprintf("- %s vs %s: %s → %s\n",
				cr.Between[0], cr.Between[1], cr.ConflictDesc, cr.Resolution)
		}
		result += "\n"
	}

	// Graph update instructions
	if len(out.GraphBatch.Instructions) > 0 {
		result += "## 图谱更新指令\n\n"
		result += "以下指令需通过 storybible_apply 写回图谱：\n\n"
		instJSON, _ := json.MarshalIndent(out.GraphBatch.Instructions, "", "  ")
		result += fmt.Sprintf("```json\n%s\n```\n\n", string(instJSON))

		result += "变更说明：\n"
		for _, desc := range storybible.DescribeBatch(out.GraphBatch, g) {
			result += fmt.Sprintf("- %s\n", desc)
		}
	}

	// LLM prompt for deeper synthesis
	result += "\n---\n"
	result += "## 完整合成提示词（供LLM深度展开）\n\n"
	result += timeline.BuildLLMPrompt(input, reactions)

	return result, nil
}

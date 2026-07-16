package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(storyBibleInit{})
	tool.RegisterBuiltin(storyBibleSnapshot{})
	tool.RegisterBuiltin(storyBibleApply{})
}

// ─── storybible_init ────────────────────────────────────────────────────────

type storyBibleInit struct{}

type storyBibleInitArgs struct {
	Characters []storyBibleInitChar `json:"characters"`
	Factions   []storyBibleInitNode `json:"factions"`
	Locations  []storyBibleInitNode `json:"locations"`
	Items      []storyBibleInitItem `json:"items"`
}

type storyBibleInitChar struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
}

type storyBibleInitNode struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
}

type storyBibleInitItem struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
}

func (s storyBibleInit) Name() string { return "storybible_init" }

func (s storyBibleInit) Description() string {
	return "初始化世界知识图谱：将结构化角色/势力/地点/物品节点写入Story Bible Graph。这是v3.0时间线生成Agent的前置步骤。至少需要一个角色节点。"
}

func (s storyBibleInit) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"characters": {
				"type": "array",
				"description": "角色节点列表",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string", "description": "角色名"},
						"properties": {"type": "object", "description": "角色属性：角色定位、性格标签(逗号分隔)、说话风格、习惯动作(逗号分隔)、核心欲望、内心恐惧、底线(逗号分隔)、当前目标(逗号分隔)、当前实力、势力等"}
					},
					"required": ["name"]
				}
			},
			"factions": {
				"type": "array",
				"description": "势力/组织节点列表",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"properties": {"type": "object", "description": "势力属性：势力范围、当前状态等"}
					},
					"required": ["name"]
				}
			},
			"locations": {
				"type": "array",
				"description": "地点节点列表",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"properties": {"type": "object", "description": "地点属性：归属、状态等"}
					},
					"required": ["name"]
				}
			},
			"items": {
				"type": "array",
				"description": "物品/功法/道具节点列表",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"properties": {"type": "object", "description": "物品属性：持有者、稀有度等"}
					},
					"required": ["name"]
				}
			}
		}
	}`)
}

func (s storyBibleInit) ReadOnly() bool { return false } // writes graph to memory

func (s storyBibleInit) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p storyBibleInitArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("storybible_init: invalid args: %w", err)
	}

	g := storybible.NewGraph()
	counts := map[string]int{}

	for _, ch := range p.Characters {
		props := ch.Properties
		if props == nil {
			props = make(map[string]interface{})
		}
		g.AddNode(storybible.Node{
			Name:       ch.Name,
			Kind:       storybible.KindCharacter,
			Properties: props,
		})
		counts["角色"]++
	}
	for _, fc := range p.Factions {
		props := fc.Properties
		if props == nil {
			props = make(map[string]interface{})
		}
		g.AddNode(storybible.Node{
			Name:       fc.Name,
			Kind:       storybible.KindFaction,
			Properties: props,
		})
		counts["势力"]++
	}
	for _, loc := range p.Locations {
		props := loc.Properties
		if props == nil {
			props = make(map[string]interface{})
		}
		g.AddNode(storybible.Node{
			Name:       loc.Name,
			Kind:       storybible.KindLocation,
			Properties: props,
		})
		counts["地点"]++
	}
	for _, it := range p.Items {
		props := it.Properties
		if props == nil {
			props = make(map[string]interface{})
		}
		g.AddNode(storybible.Node{
			Name:       it.Name,
			Kind:       storybible.KindItem,
			Properties: props,
		})
		counts["物品"]++
	}

	// Serialise graph to JSON for persistence by the caller
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", fmt.Errorf("storybible_init: marshal error: %w", err)
	}

	total := 0
	var parts []string
	for k, v := range counts {
		parts = append(parts, fmt.Sprintf("%s: %d", k, v))
		total += v
	}

	summary := fmt.Sprintf("Story Bible Graph 初始化完成。总计 %d 个节点（%s）。\n\n请将以下JSON保存到 .novel-agent/story-bible.json：\n\n```json\n%s\n```\n\n下一步：使用 storybible_snapshot 为特定角色生成子图谱，然后使用 character_react 和 timeline_synthesize 进行时间线生成。",
		total, joinParts(parts), string(data))

	return summary, nil
}

func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "，" + parts[i]
	}
	return result
}

// ─── storybible_snapshot ────────────────────────────────────────────────────

type storyBibleSnapshot struct{}

type storyBibleSnapshotArgs struct {
	GraphJSON    string   `json:"graph_json"`    // story-bible.json 的内容
	CharacterIDs []string `json:"character_ids"` // 要包含的角色ID列表
	Depth        int      `json:"depth"`         // 扩展深度（默认1）
}

func (s storyBibleSnapshot) Name() string { return "storybible_snapshot" }

func (s storyBibleSnapshot) Description() string {
	return "从世界知识图谱中生成局部子图谱快照。传入图谱JSON和角色ID列表，返回这些角色及其直接关系节点的子视图。用于控制角色Agent的上下文长度。"
}

func (s storyBibleSnapshot) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"graph_json": {"type": "string", "description": "story-bible.json 的完整JSON内容（先用 read_file 读取）"},
			"character_ids": {"type": "array", "items": {"type": "string"}, "description": "要生成快照的角色节点ID列表"},
			"depth": {"type": "integer", "description": "扩展深度：0=仅种子节点，1=种子节点+直接邻居+相关边（默认1）"}
		},
		"required": ["graph_json", "character_ids"]
	}`)
}

func (s storyBibleSnapshot) ReadOnly() bool { return true }

func (s storyBibleSnapshot) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p storyBibleSnapshotArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("storybible_snapshot: invalid args: %w", err)
	}
	if p.Depth <= 0 {
		p.Depth = 1
	}

	g := storybible.NewGraph()
	if err := json.Unmarshal([]byte(p.GraphJSON), g); err != nil {
		return "", fmt.Errorf("storybible_snapshot: invalid graph JSON: %w", err)
	}

	snap := g.SnapshotFor(p.CharacterIDs, p.Depth)
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("storybible_snapshot: marshal error: %w", err)
	}

	summary := fmt.Sprintf("子图谱快照：%d 个节点，%d 条边（种子角色: %v，深度: %d）\n\n```json\n%s\n```",
		len(snap.Nodes), len(snap.Edges), p.CharacterIDs, p.Depth, string(data))

	return summary, nil
}

// ─── storybible_apply ───────────────────────────────────────────────────────

type storyBibleApply struct{}

type storyBibleApplyArgs struct {
	GraphJSON    string                         `json:"graph_json"`   // story-bible.json 的内容
	Instructions []storybible.UpdateInstruction `json:"instructions"` // 更新指令列表
	Chapter      int                            `json:"chapter"`      // 章节号
}

func (s storyBibleApply) Name() string { return "storybible_apply" }

func (s storyBibleApply) Description() string {
	return "将图谱更新指令应用到世界知识图谱。每章写完后，timeline_synthesize 工具输出更新指令，用此工具将变更写回图谱。"
}

func (s storyBibleApply) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"graph_json": {"type": "string", "description": "story-bible.json 的完整JSON内容"},
			"instructions": {
				"type": "array",
				"description": "更新指令列表（由 timeline_synthesize 输出）",
				"items": {
					"type": "object",
					"properties": {
						"op": {"type": "string", "enum": ["set_prop", "add_edge", "upsert_edge", "remove_edge"]},
						"node_id": {"type": "string"},
						"from_id": {"type": "string"},
						"to_id": {"type": "string"},
						"rel_kind": {"type": "string"},
						"edge_id": {"type": "string"},
						"props": {"type": "object"},
						"reason": {"type": "string"}
					},
					"required": ["op"]
				}
			},
			"chapter": {"type": "integer", "description": "触发更新的章节号"}
		},
		"required": ["graph_json", "instructions"]
	}`)
}

func (s storyBibleApply) ReadOnly() bool { return false }

func (s storyBibleApply) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p storyBibleApplyArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("storybible_apply: invalid args: %w", err)
	}

	g := storybible.NewGraph()
	if err := json.Unmarshal([]byte(p.GraphJSON), g); err != nil {
		return "", fmt.Errorf("storybible_apply: invalid graph JSON: %w", err)
	}

	batch := storybible.UpdateBatch{
		Chapter:      p.Chapter,
		Instructions: p.Instructions,
	}

	errs := g.ApplyBatch(batch)
	if len(errs) > 0 {
		var errMsgs []string
		for _, e := range errs {
			errMsgs = append(errMsgs, e.Error())
		}
		return "", fmt.Errorf("storybible_apply: %d errors: %v", len(errs), errMsgs)
	}

	// Serialise updated graph
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", fmt.Errorf("storybible_apply: marshal error: %w", err)
	}

	// Generate change log
	changes := storybible.DescribeBatch(batch, g)
	summary := fmt.Sprintf("图谱更新完成。第%d章，%d条变更：\n", p.Chapter, len(p.Instructions))
	for _, ch := range changes {
		summary += fmt.Sprintf("- %s\n", ch)
	}
	summary += fmt.Sprintf("\n请覆盖保存到 .novel-agent/story-bible.json：\n\n```json\n%s\n```", string(data))

	return summary, nil
}

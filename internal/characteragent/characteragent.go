// Package characteragent provides the Character Personality Agent — a
// per-character reasoning unit that generates independent, personality-consistent
// reactions to story events. Each agent runs in isolation, consuming a snapshot
// of the Story Bible Graph and producing structured reaction output.
//
// Key design invariants (from the design doc):
//   - Character agents do NOT perceive each other's full outputs. They only
//     perceive graph-confirmed facts. This prevents "mind-reading" logic collapse.
//   - Real interactions between characters only land through graph updates.
//   - Each agent output includes: internal reaction, action tendencies (with
//     priority), and whether new relationship changes are triggered.
package characteragent

import (
	"fmt"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
)

// ─── Character Profile ─────────────────────────────────────────────────────

// CharacterProfile is the personality constraint template for one character.
// It is derived from the Story Bible Graph character node + additional
// author-defined constraints. This is what gets injected into each character
// agent's system prompt.
type CharacterProfile struct {
	NodeID       string   `json:"node_id"`       // references storybible.Graph node
	Name         string   `json:"name"`
	Role         string   `json:"role"`           // 主角/反派/配角/路人
	Personality  []string `json:"personality"`    // 性格标签 (≥3正向 + ≥1缺陷)
	SpeechStyle  string   `json:"speech_style"`   // 说话风格/口头禅
	Habits       []string `json:"habits"`         // 习惯性动作
	CoreDesire   string   `json:"core_desire"`    // 核心欲望
	InnerFear    string   `json:"inner_fear"`     // 内心恐惧
	RedLines     []string `json:"red_lines"`      // 绝不会做的事 (底线)
	Goals        []string `json:"goals"`          // 当前阶段目标
	CurrentState string   `json:"current_state"`  // 当前状态 (从graph同步)
}

// DeriveProfileFromNode creates a CharacterProfile from a storybible character node.
func DeriveProfileFromNode(node *storybible.Node) *CharacterProfile {
	if node == nil || node.Kind != storybible.KindCharacter {
		return nil
	}
	p := &CharacterProfile{
		NodeID: node.ID,
		Name:   node.Name,
	}
	if v, ok := node.Properties["角色定位"]; ok {
		p.Role = fmt.Sprint(v)
	}
	if v, ok := node.Properties["性格标签"]; ok {
		p.Personality = splitCSV(fmt.Sprint(v))
	}
	if v, ok := node.Properties["说话风格"]; ok {
		p.SpeechStyle = fmt.Sprint(v)
	}
	if v, ok := node.Properties["习惯动作"]; ok {
		p.Habits = splitCSV(fmt.Sprint(v))
	}
	if v, ok := node.Properties["核心欲望"]; ok {
		p.CoreDesire = fmt.Sprint(v)
	}
	if v, ok := node.Properties["内心恐惧"]; ok {
		p.InnerFear = fmt.Sprint(v)
	}
	if v, ok := node.Properties["底线"]; ok {
		p.RedLines = splitCSV(fmt.Sprint(v))
	}
	if v, ok := node.Properties["当前目标"]; ok {
		p.Goals = splitCSV(fmt.Sprint(v))
	}
	if v, ok := node.Properties["当前状态"]; ok {
		p.CurrentState = fmt.Sprint(v)
	}
	return p
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// RenderSystemPrompt renders the character profile as an LLM system prompt.
func (p *CharacterProfile) RenderSystemPrompt() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【角色锁定】你现在是《%s》（%s）。", p.Name, p.Role))
	b.WriteString("你必须完全从这个角色的视角思考，不允许跳出角色。\n\n")

	b.WriteString("## 性格\n")
	for _, tag := range p.Personality {
		b.WriteString(fmt.Sprintf("- %s\n", tag))
	}

	if p.SpeechStyle != "" {
		b.WriteString(fmt.Sprintf("\n## 说话风格\n%s\n", p.SpeechStyle))
	}
	if len(p.Habits) > 0 {
		b.WriteString("\n## 习惯动作\n")
		for _, h := range p.Habits {
			b.WriteString(fmt.Sprintf("- %s\n", h))
		}
	}
	if p.CoreDesire != "" {
		b.WriteString(fmt.Sprintf("\n## 核心欲望\n%s\n", p.CoreDesire))
	}
	if p.InnerFear != "" {
		b.WriteString(fmt.Sprintf("\n## 内心恐惧\n%s\n", p.InnerFear))
	}
	if len(p.RedLines) > 0 {
		b.WriteString("\n## 绝对不会做的事\n")
		for _, r := range p.RedLines {
			b.WriteString(fmt.Sprintf("- %s\n", r))
		}
	}
	if len(p.Goals) > 0 {
		b.WriteString("\n## 当前目标\n")
		for _, g := range p.Goals {
			b.WriteString(fmt.Sprintf("- %s\n", g))
		}
	}
	b.WriteString("\n## 输出格式要求\n")
	b.WriteString("你必须严格按照以下YAML格式输出，不要输出任何额外内容：\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("内心反应: |\n")
	b.WriteString("  (对角色的内心判断、情绪反应、推理过程)\n")
	b.WriteString("行动倾向:\n")
	b.WriteString("  - 行动: (具体行动描述)\n")
	b.WriteString("    优先级: 高|中|低\n")
	b.WriteString("    触发条件: (什么情况下会执行)\n")
	b.WriteString("关系变化:\n")
	b.WriteString("  - 目标角色: (角色名)\n")
	b.WriteString("    变化: (关系如何改变)\n")
	b.WriteString("    原因: (触发改变的原因)\n")
	b.WriteString("```\n")
	return b.String()
}

// ─── Memory Summary ─────────────────────────────────────────────────────────

// MemorySummary is a compressed summary of recent key events for a character.
// It replaces full chapter history to control context length.
type MemorySummary struct {
	CharacterID  string    `json:"character_id"`
	RecentEvents []string  `json:"recent_events"`   // 最近1-3章关键事件摘要 (每条约50字)
	LastUpdated  time.Time `json:"last_updated"`
	ChapterRange string    `json:"chapter_range"`   // e.g. "第12-14章"
}

// Render renders the memory summary as a compact text block.
func (m *MemorySummary) Render() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 近期记忆 (%s)\n", m.ChapterRange))
	for _, e := range m.RecentEvents {
		b.WriteString(fmt.Sprintf("- %s\n", e))
	}
	return b.String()
}

// ─── Trigger Event ──────────────────────────────────────────────────────────

// TriggerEvent describes the event that triggers character reactions.
type TriggerEvent struct {
	EventType    string   `json:"event_type"`     // 事件类型: 战斗/对话/发现/背叛/...
	Description  string   `json:"description"`    // 事件描述
	InvolvedChars []string `json:"involved_chars"` // 涉及的其他角色名
	Location     string   `json:"location"`       // 发生地点
	ChapterNum   int      `json:"chapter_num"`    // 所在章节
}

// Render renders the trigger event as a prompt segment.
func (te *TriggerEvent) Render() string {
	var b strings.Builder
	b.WriteString("## 当前触发事件\n")
	b.WriteString(fmt.Sprintf("事件类型: %s\n", te.EventType))
	b.WriteString(fmt.Sprintf("事件描述: %s\n", te.Description))
	if len(te.InvolvedChars) > 0 {
		b.WriteString(fmt.Sprintf("涉及角色: %s\n", strings.Join(te.InvolvedChars, "、")))
	}
	if te.Location != "" {
		b.WriteString(fmt.Sprintf("地点: %s\n", te.Location))
	}
	return b.String()
}

// ─── Reaction (Agent Output) ────────────────────────────────────────────────

// Reaction is the structured output of one character agent's processing.
type Reaction struct {
	CharacterID    string           `json:"character_id"`
	CharacterName  string           `json:"character_name"`
	InternalThought string          `json:"internal_thought"`  // 内心反应/判断
	Actions        []ActionCandidate `json:"actions"`          // 行动倾向 (按优先级排序)
	RelationshipChanges []RelChange  `json:"relationship_changes"` // 关系变化
	GeneratedAt    time.Time        `json:"generated_at"`
}

// ActionCandidate is one possible action with priority.
type ActionCandidate struct {
	Action      string `json:"action"`       // 具体行动描述
	Priority    string `json:"priority"`     // 高/中/低
	TriggerCond string `json:"trigger_cond"` // 触发条件
}

// RelChange describes a potential relationship state change.
type RelChange struct {
	TargetName string `json:"target_name"` // 目标角色名
	Change     string `json:"change"`      // 关系如何改变
	Reason     string `json:"reason"`      // 触发原因
}

// Render renders the reaction as a human-readable summary.
func (r *Reaction) Render() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s】的反应:\n", r.CharacterName))
	b.WriteString(fmt.Sprintf("  内心: %s\n", truncate(r.InternalThought, 120)))
	if len(r.Actions) > 0 {
		b.WriteString("  行动倾向:\n")
		for _, a := range r.Actions {
			b.WriteString(fmt.Sprintf("    [%s] %s", a.Priority, a.Action))
			if a.TriggerCond != "" {
				b.WriteString(fmt.Sprintf(" (触发: %s)", a.TriggerCond))
			}
			b.WriteString("\n")
		}
	}
	if len(r.RelationshipChanges) > 0 {
		b.WriteString("  关系变化:\n")
		for _, rc := range r.RelationshipChanges {
			b.WriteString(fmt.Sprintf("    → %s: %s (%s)\n", rc.TargetName, rc.Change, rc.Reason))
		}
	}
	return b.String()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// ─── Agent ──────────────────────────────────────────────────────────────────

// Agent represents one character's reasoning unit. It is call-site
// responsible for calling the LLM; this package only prepares the prompt
// context. The actual LLM call is orchestrated by the Timeline Synthesis Agent.
type Agent struct {
	Profile      *CharacterProfile
	Memory       *MemorySummary
	GraphSnap    *storybible.Snapshot // the subgraph this character perceives
}

// BuildPrompt assembles the full prompt context for one character agent call.
// It includes: system prompt (character profile), graph snapshot (world state),
// memory summary, and trigger event.
func (a *Agent) BuildPrompt(event TriggerEvent) string {
	var b strings.Builder

	// 1. System prompt: character lock + personality
	b.WriteString(a.Profile.RenderSystemPrompt())
	b.WriteString("\n\n")

	// 2. World state snapshot (graph subview)
	if a.GraphSnap != nil {
		b.WriteString("## 当前世界状态（你已知的客观事实）\n")
		b.WriteString(renderSnapshot(a.GraphSnap, a.Profile.NodeID))
		b.WriteString("\n")
	}

	// 3. Recent memory summary
	if a.Memory != nil {
		b.WriteString(a.Memory.Render())
		b.WriteString("\n")
	}

	// 4. Trigger event
	b.WriteString(event.Render())
	b.WriteString("\n")

	// 5. Final instruction
	b.WriteString("请基于以上信息，按格式输出你（该角色）在此事件下的反应。")
	b.WriteString("记住：你只能感知图谱中已确认的客观事实，不能读其他角色的内心。\n")

	return b.String()
}

// renderSnapshot renders the graph snapshot for character consumption.
func renderSnapshot(snap *storybible.Snapshot, selfID string) string {
	var b strings.Builder
	for id, n := range snap.Nodes {
		if id == selfID {
			continue // skip self; already in system prompt
		}
		b.WriteString(fmt.Sprintf("- %s (%s)", n.Name, n.Kind))
		// Show relevant properties
		props := relevantProps(n)
		if len(props) > 0 {
			b.WriteString(": ")
			b.WriteString(strings.Join(props, ", "))
		}
		b.WriteString("\n")
	}
	// Show relevant edges (only those connected to self)
	for _, e := range snap.Edges {
		if e.From == selfID || e.To == selfID {
			otherID := e.To
			if e.To == selfID {
				otherID = e.From
			}
			otherName := otherID
			if n, ok := snap.Nodes[otherID]; ok {
				otherName = n.Name
			}
			b.WriteString(fmt.Sprintf("  关系: → %s (%s)", otherName, e.Kind))
			if reason, ok := e.Properties["reason"]; ok {
				b.WriteString(fmt.Sprintf(" [%s]", reason))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func relevantProps(n *storybible.Node) []string {
	// Show only narrative-relevant properties, filtered by node kind.
	relevant := map[storybible.NodeKind][]string{
		storybible.KindCharacter: {"角色定位", "当前状态", "当前实力", "势力"},
		storybible.KindFaction:   {"势力范围", "当前状态"},
		storybible.KindLocation:  {"状态", "归属"},
		storybible.KindItem:      {"持有者", "稀有度", "当前状态"},
	}
	keys := relevant[n.Kind]
	var props []string
	for _, k := range keys {
		if v, ok := n.Properties[k]; ok {
			props = append(props, fmt.Sprintf("%s: %v", k, v))
		}
	}
	return props
}

// ─── Batch Runner ───────────────────────────────────────────────────────────

// BatchSpec describes a batch of character agents to run for one event.
type BatchSpec struct {
	Profiles   []*CharacterProfile         // one per character
	Memories   map[string]*MemorySummary   // character ID → memory
	GraphSnap  *storybible.Snapshot        // shared subgraph (depth 1)
	Event      TriggerEvent                // the trigger event
}

// BuildAllPrompts builds the prompt for every character in the batch.
// Returns character ID → assembled prompt.
func (bs *BatchSpec) BuildAllPrompts() map[string]string {
	out := make(map[string]string, len(bs.Profiles))
	for _, p := range bs.Profiles {
		a := &Agent{
			Profile:   p,
			GraphSnap: bs.GraphSnap,
		}
		if bs.Memories != nil {
			a.Memory = bs.Memories[p.NodeID]
		}
		out[p.NodeID] = a.BuildPrompt(bs.Event)
	}
	return out
}

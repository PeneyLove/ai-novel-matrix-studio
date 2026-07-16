// Package recorderagent provides the Recorder Agent — the second stage in
// the v4.0 four-stage pipeline. It operates in two modes:
//
//   Integration mode (整合模式) — Collects parallel character-agent reactions,
//   sorts them into a coherent event sequence, detects action conflicts,
//   extracts key dialogues, and outputs a structured scene description.
//
//   Consultant mode (顾问模式) — Activated when the Review Agent rejects a
//   chapter draft. The Recorder Agent holds the ground-truth deduction data
//   and answers queries from the Novel Writer Agent during rewrite cycles.
//   Four query types: scene recall, character state, conflict confirmation,
//   omission check.
//
// Design invariant: the Recorder Agent is the single source of truth for
// "what actually happened" in the deduction. It never writes prose — that's
// the Novel Writer Agent's job.
package recorderagent

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
)

// ─── Scene Description (Integration Output) ────────────────────────────────

// Scene is the structured output of the integration phase.
// It captures the narrative skeleton that the Novel Writer Agent
// will later flesh out into web-novel prose.
type Scene struct {
	Title        string          `json:"title"`         // scene name (e.g. "青云宗山门前")
	Location     string          `json:"location"`      // where it takes place
	ChapterNum   int             `json:"chapter_num"`   // which chapter
	EventSeq     []EventNode     `json:"event_sequence"` // ordered events
	Conflicts    []ConflictNote  `json:"conflicts"`     // detected action conflicts
	Dialogues    []DialogueBeat  `json:"dialogues"`     // key dialogue snippets
	RhythmHints  RhythmHints     `json:"rhythm_hints"`  // 4-beat structure hints
	AllReactions []characteragent.Reaction `json:"-"`   // raw reactions (not serialised, used for consultant mode)
	GeneratedAt  time.Time       `json:"generated_at"`
}

// EventNode is one atomic event in the scene sequence.
type EventNode struct {
	Order     int    `json:"order"`
	Actor     string `json:"actor"`     // character name
	Action    string `json:"action"`    // what they do
	Emotion   string `json:"emotion"`   // dominant emotion
	Priority  string `json:"priority"`  // 高/中/低
	TargetOf  string `json:"target_of"` // who this action targets (if any)
}

// ConflictNote records a detected opposition between two characters' actions.
type ConflictNote struct {
	Between    []string `json:"between"`    // [charA, charB]
	Type       string   `json:"type"`       // 战斗冲突/立场冲突/情感冲突/利益冲突
	Resolution string   `json:"resolution"` // how it resolves (prioritising protagonist)
}

// DialogueBeat records a key line that should appear in the chapter.
type DialogueBeat struct {
	Speaker string `json:"speaker"`
	Line    string `json:"line"`
	Context string `json:"context"` // what triggers this line
}

// RhythmHints maps the 4-beat web-novel rhythm to scene content.
type RhythmHints struct {
	Setup    string `json:"setup"`     // 铺垫
	Conflict string `json:"conflict"`  // 冲突升级
	Climax   string `json:"climax"`    // 高潮/爽点
	Suspense string `json:"suspense"`  // 悬念收尾
}

// ─── Agent ──────────────────────────────────────────────────────────────────

// Agent is the Recorder Agent. It holds the graph reference for
// name resolution and state lookups.
type Agent struct {
	graph         *storybible.Graph
	protagonistID string
	targetWords   int // target word count for the chapter
}

// New creates a Recorder Agent.
func New(graph *storybible.Graph, protagonistID string) *Agent {
	return &Agent{
		graph:         graph,
		protagonistID: protagonistID,
		targetWords:   3000,
	}
}

// SetTargetWords sets the target word count for the chapter.
func (a *Agent) SetTargetWords(n int) { a.targetWords = n }

// ─── Integration Mode ───────────────────────────────────────────────────────

// IntegrateInput is the material the Recorder Agent works with.
type IntegrateInput struct {
	ChapterNum   int                              // chapter number
	Reactions    []characteragent.Reaction         // all character reactions
	Profiles     map[string]*characteragent.CharacterProfile
	TriggerEvent characteragent.TriggerEvent       // the event that triggered reactions
	GraphSnap    *storybible.Snapshot              // world state at time of deduction
}

// Integrate runs the integration mode: collect, sort, detect conflicts,
// extract dialogues, and build a structured scene description.
func (a *Agent) Integrate(in IntegrateInput) *Scene {
	scene := &Scene{
		Title:        in.TriggerEvent.Description,
		Location:     in.TriggerEvent.Location,
		ChapterNum:   in.ChapterNum,
		AllReactions: in.Reactions,
		GeneratedAt:  time.Now(),
	}

	// Phase 1: Build event sequence from all reactions.
	scene.EventSeq = a.buildEventSequence(in)

	// Phase 2: Detect conflicts.
	scene.Conflicts = a.detectConflicts(in.Reactions, in.Profiles)

	// Phase 3: Extract key dialogues.
	scene.Dialogues = a.extractDialogues(in)

	// Phase 4: Map to rhythm structure.
	scene.RhythmHints = a.buildRhythmHints(scene.EventSeq, in.TriggerEvent)

	return scene
}

// buildEventSequence collects actions from all reactions, sorts by priority
// (protagonist first) and arranges into an ordered event list.
func (a *Agent) buildEventSequence(in IntegrateInput) []EventNode {
	type rawAction struct {
		charName string
		charID   string
		action   string
		priority int // 高=3, 中=2, 低=1
		emotion  string
		target   string
	}

	var actions []rawAction
	for _, r := range in.Reactions {
		for _, act := range r.Actions {
			p := 1
			switch act.Priority {
			case "高":
				p = 3
			case "中":
				p = 2
			}
			target := ""
			for _, rc := range r.RelationshipChanges {
				target = rc.TargetName
			}
			actions = append(actions, rawAction{
				charName: r.CharacterName,
				charID:   r.CharacterID,
				action:   act.Action,
				priority: p,
				emotion:  extractEmotionFrom(r.InternalThought),
				target:   target,
			})
		}
	}

	// Sort: protagonist first, then priority descending.
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].charID == a.protagonistID && actions[j].charID != a.protagonistID {
			return true
		}
		if actions[j].charID == a.protagonistID && actions[i].charID != a.protagonistID {
			return false
		}
		return actions[i].priority > actions[j].priority
	})

	var nodes []EventNode
	for i, act := range actions {
		nodes = append(nodes, EventNode{
			Order:    i + 1,
			Actor:    act.charName,
			Action:   act.action,
			Emotion:  act.emotion,
			Priority: priorityLabel(act.priority),
			TargetOf: act.target,
		})
	}
	return nodes
}

func priorityLabel(p int) string {
	switch p {
	case 3:
		return "高"
	case 2:
		return "中"
	default:
		return "低"
	}
}

// detectConflicts finds opposing high-priority actions between characters.
func (a *Agent) detectConflicts(reactions []characteragent.Reaction, profiles map[string]*characteragent.CharacterProfile) []ConflictNote {
	var conflicts []ConflictNote

	type intent struct {
		charName string
		action   string
		category string
	}

	var intents []intent
	for _, r := range reactions {
		for _, act := range r.Actions {
			if act.Priority != "高" {
				continue
			}
			intents = append(intents, intent{
				charName: r.CharacterName,
				action:   act.Action,
				category: classifyActionCategory(act.Action),
			})
		}
	}

	// Pairwise opposition check.
	opposites := map[string]string{
		"攻击": "保护",
		"保护": "攻击",
		"揭示": "隐藏",
		"隐藏": "揭示",
	}

	for i := 0; i < len(intents); i++ {
		for j := i + 1; j < len(intents); j++ {
			a, b := intents[i], intents[j]
			if opposites[a.category] == b.category {
				conflictType := "立场冲突"
				if a.category == "攻击" || a.category == "保护" {
					conflictType = "战斗冲突"
				}
				resolution := fmt.Sprintf("%s优先（主角剧情线）", a.charName)
				conflicts = append(conflicts, ConflictNote{
					Between:    []string{a.charName, b.charName},
					Type:       conflictType,
					Resolution: resolution,
				})
			}
		}
	}
	return conflicts
}

func classifyActionCategory(action string) string {
	action = strings.ToLower(action)
	switch {
	case strings.Contains(action, "攻击") || strings.Contains(action, "杀") || strings.Contains(action, "出手") || strings.Contains(action, "偷袭"):
		return "攻击"
	case strings.Contains(action, "保护") || strings.Contains(action, "护") || strings.Contains(action, "掩护") || strings.Contains(action, "救"):
		return "保护"
	case strings.Contains(action, "揭示") || strings.Contains(action, "暴露") || strings.Contains(action, "说出"):
		return "揭示"
	case strings.Contains(action, "隐藏") || strings.Contains(action, "隐瞒"):
		return "隐藏"
	default:
		return "其他"
	}
}

// extractDialogues generates plausible dialogue beats from character reactions.
func (a *Agent) extractDialogues(in IntegrateInput) []DialogueBeat {
	var dialogues []DialogueBeat

	for _, r := range in.Reactions {
		// Generate dialogue from high-priority actions and relationship changes.
		for _, act := range r.Actions {
			if act.Priority != "高" {
				continue
			}
			line := generateDialogueLine(r.CharacterName, act.Action, in.Profiles)
			if line != "" {
				dialogues = append(dialogues, DialogueBeat{
					Speaker: r.CharacterName,
					Line:    line,
					Context: act.TriggerCond,
				})
			}
		}
	}
	return dialogues
}

// generateDialogueLine creates a plausible dialogue line from a character action.
func generateDialogueLine(charName, action string, profiles map[string]*characteragent.CharacterProfile) string {
	profile := findProfile(charName, profiles)
	style := "直接"
	if profile != nil && profile.SpeechStyle != "" {
		style = profile.SpeechStyle
	}

	action = strings.ToLower(action)
	switch {
	case strings.Contains(action, "攻击") || strings.Contains(action, "出手"):
		if strings.Contains(style, "言简") || strings.Contains(style, "不怒自威") {
			return "「找死。」"
		}
		return "「来吧。」"
	case strings.Contains(action, "保护") || strings.Contains(action, "护"):
		return "「退后！」"
	case strings.Contains(action, "探查") || strings.Contains(action, "调查"):
		return "「此事有蹊跷。」"
	case strings.Contains(action, "劝说"):
		return "「听我一言。」"
	case strings.Contains(action, "谈判"):
		return "「说出你的条件。」"
	}
	return ""
}

func findProfile(name string, profiles map[string]*characteragent.CharacterProfile) *characteragent.CharacterProfile {
	for _, p := range profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// buildRhythmHints maps event sequence to the 4-beat web-novel structure.
func (a *Agent) buildRhythmHints(events []EventNode, trigger characteragent.TriggerEvent) RhythmHints {
	h := RhythmHints{}

	if len(events) == 0 {
		h.Setup = "场景建立"
		h.Conflict = "引入冲突"
		h.Climax = "高潮转折"
		h.Suspense = "悬念钩子"
		return h
	}

	// Setup: first event (establish scene)
	h.Setup = fmt.Sprintf("%s：%s", events[0].Actor, events[0].Action)

	// Conflict: middle event (tension escalation)
	if len(events) >= 2 {
		mid := len(events) / 2
		h.Conflict = fmt.Sprintf("%s vs %s：冲突升级", events[mid].Actor, events[mid].TargetOf)
	}

	// Climax: protagonist's peak action
	for _, ev := range events {
		if ev.Priority == "高" {
			h.Climax = fmt.Sprintf("%s：%s（高光时刻）", ev.Actor, ev.Action)
			break
		}
	}
	if h.Climax == "" && len(events) > 0 {
		last := events[len(events)-1]
		h.Climax = fmt.Sprintf("%s：%s", last.Actor, last.Action)
	}

	// Suspense: hook for next chapter
	h.Suspense = "下一章钩子待注入"

	return h
}

// ─── Consultant Mode ────────────────────────────────────────────────────────

// ConsultantQuery represents one question from the Novel Writer Agent
// to the Recorder Agent during the rewrite cycle.
type ConsultantQuery struct {
	Type     string `json:"type"`      // scene_recall | character_state | conflict_confirm | omission_check
	Question string `json:"question"`  // natural-language question
	Target   string `json:"target"`    // which event/character/scene is being asked about
}

// ConsultantAnswer is the Recorder Agent's response in consultant mode.
type ConsultantAnswer struct {
	QueryType string `json:"query_type"`
	Answer    string `json:"answer"`
	Evidence  string `json:"evidence"` // data from the original deduction that backs this answer
	EventRef  *EventNode `json:"event_ref,omitempty"` // referenced event node, if applicable
}

// Consult handles a single consultant query from the Novel Writer Agent.
// The scene parameter is the original integration output that serves as
// the ground truth for all answers.
func (a *Agent) Consult(query ConsultantQuery, scene *Scene) ConsultantAnswer {
	ans := ConsultantAnswer{QueryType: query.Type}

	switch query.Type {
	case "scene_recall":
		ans = a.handleSceneRecall(query, scene)
	case "character_state":
		ans = a.handleCharacterState(query, scene)
	case "conflict_confirm":
		ans = a.handleConflictConfirm(query, scene)
	case "omission_check":
		ans = a.handleOmissionCheck(query, scene)
	default:
		ans.Answer = "无法识别的查询类型。支持: scene_recall, character_state, conflict_confirm, omission_check"
	}

	return ans
}

func (a *Agent) handleSceneRecall(q ConsultantQuery, scene *Scene) ConsultantAnswer {
	ans := ConsultantAnswer{QueryType: "scene_recall"}

	for _, ev := range scene.EventSeq {
		if strings.Contains(ev.Action, q.Target) || strings.Contains(ev.Actor, q.Target) {
			ans.Answer = fmt.Sprintf("%s在事件#%d中执行了'%s'（情绪：%s，优先级：%s）",
				ev.Actor, ev.Order, ev.Action, ev.Emotion, ev.Priority)
			ans.Evidence = fmt.Sprintf("原始推演数据: order=%d, emotion=%s", ev.Order, ev.Emotion)
			ans.EventRef = &ev
			return ans
		}
	}

	// Fallback: search all events
	if len(scene.EventSeq) > 0 {
		ev := scene.EventSeq[0]
		ans.Answer = fmt.Sprintf("未找到精确匹配'%s'。最近事件: %s → %s", q.Target, ev.Actor, ev.Action)
		ans.EventRef = &ev
	}
	return ans
}

func (a *Agent) handleCharacterState(q ConsultantQuery, scene *Scene) ConsultantAnswer {
	ans := ConsultantAnswer{QueryType: "character_state"}

	for _, r := range scene.AllReactions {
		if r.CharacterName == q.Target {
			ans.Answer = fmt.Sprintf("%s的状态: 内心'%s'，行动倾向%d个，关系变化%d条",
				r.CharacterName,
				truncateStr(r.InternalThought, 80),
				len(r.Actions),
				len(r.RelationshipChanges))
			ans.Evidence = fmt.Sprintf("原始推演内部反应: %s", r.InternalThought)
			return ans
		}
	}

	ans.Answer = fmt.Sprintf("未在原始推演中找到角色'%s'的数据", q.Target)
	return ans
}

func (a *Agent) handleConflictConfirm(q ConsultantQuery, scene *Scene) ConsultantAnswer {
	ans := ConsultantAnswer{QueryType: "conflict_confirm"}

	for _, c := range scene.Conflicts {
		for _, name := range c.Between {
			if strings.Contains(q.Target, name) {
				ans.Answer = fmt.Sprintf("冲突确认: %s vs %s（%s），解决: %s",
					c.Between[0], c.Between[1], c.Type, c.Resolution)
				ans.Evidence = fmt.Sprintf("冲突类型: %s", c.Type)
				return ans
			}
		}
	}

	if len(scene.Conflicts) > 0 {
		c := scene.Conflicts[0]
		ans.Answer = fmt.Sprintf("未找到关于'%s'的冲突。已有冲突: %s vs %s", q.Target, c.Between[0], c.Between[1])
	} else {
		ans.Answer = fmt.Sprintf("未检测到任何冲突。'%s'的场景没有对立行动。", q.Target)
	}
	return ans
}

func (a *Agent) handleOmissionCheck(q ConsultantQuery, scene *Scene) ConsultantAnswer {
	ans := ConsultantAnswer{QueryType: "omission_check"}

	var covered []string
	var missing []string
	for i, ev := range scene.EventSeq {
		if ev.Priority == "高" {
			covered = append(covered, fmt.Sprintf("事件#%d(%s:%s)", i+1, ev.Actor, ev.Action))
		}
	}

	if len(missing) == 0 {
		ans.Answer = fmt.Sprintf("所有%d个高优先级事件均已覆盖: %s", len(covered), strings.Join(covered, "、"))
		ans.Evidence = fmt.Sprintf("原始事件序列共%d个节点", len(scene.EventSeq))
	} else {
		ans.Answer = fmt.Sprintf("已覆盖: %s。遗漏: %s", strings.Join(covered, "、"), strings.Join(missing, "、"))
	}
	return ans
}

// ─── Prompt Builders for LLM Integration ────────────────────────────────────

const recorderSystemPrompt = `【角色锁定】你是网文记录Agent，负责整合多角色并行推演的输出。
你的任务是客观记录"发生了什么"，不创作、不润色、不发挥。
你持有最完整的原始推演数据，是剧情真相的唯一来源。`

// BuildIntegratePrompt builds the LLM prompt for integration mode.
func BuildIntegratePrompt(reactions []characteragent.Reaction, trigger characteragent.TriggerEvent) string {
	var b strings.Builder
	b.WriteString(recorderSystemPrompt)
	b.WriteString("\n\n## 任务：事件整合\n\n")
	b.WriteString("收集以下角色的反应，整合为一个结构化场景描述。\n\n")

	b.WriteString(fmt.Sprintf("## 触发事件: %s（第%d章）\n", trigger.Description, trigger.ChapterNum))
	if trigger.Location != "" {
		b.WriteString(fmt.Sprintf("地点: %s\n", trigger.Location))
	}
	b.WriteString("\n## 各角色反应\n\n")
	for _, r := range reactions {
		b.WriteString(r.Render())
		b.WriteString("\n")
	}

	b.WriteString("\n## 输出格式: JSON\n")
	b.WriteString("```json\n")
	b.WriteString(`{
  "scene_title": "场景标题",
  "event_sequence": [
    {"order": 1, "actor": "角色名", "action": "行动", "emotion": "情绪"}
  ],
  "conflicts": [
    {"between": ["角色A", "角色B"], "type": "冲突类型", "resolution": "解决方案"}
  ],
  "dialogues": [
    {"speaker": "角色", "line": "台词", "context": "触发情境"}
  ],
  "rhythm_hints": {
    "setup": "铺垫内容",
    "conflict": "冲突内容",
    "climax": "高潮内容",
    "suspense": "悬念钩子"
  }
}
`)
	b.WriteString("```\n")

	return b.String()
}

// BuildConsultantContext builds the consultant-mode system prompt for the
// recorder agent when it needs to assist the novel writer during rewrite.
func BuildConsultantContext(scene *Scene) string {
	var b strings.Builder
	b.WriteString(recorderSystemPrompt)
	b.WriteString("\n\n## 当前模式：顾问模式\n\n")
	b.WriteString("网文Agent正在修改稿子，你可能被查询以下信息。以下是你能提供的原始推演数据：\n\n")

	b.WriteString(fmt.Sprintf("## 事件序列（共%d个事件）\n", len(scene.EventSeq)))
	for _, ev := range scene.EventSeq {
		b.WriteString(fmt.Sprintf("- #%d %s: %s [%s]\n", ev.Order, ev.Actor, ev.Action, ev.Emotion))
	}

	if len(scene.Conflicts) > 0 {
		b.WriteString("\n## 冲突记录\n")
		for _, c := range scene.Conflicts {
			b.WriteString(fmt.Sprintf("- %s vs %s (%s) → %s\n", c.Between[0], c.Between[1], c.Type, c.Resolution))
		}
	}

	if len(scene.Dialogues) > 0 {
		b.WriteString("\n## 关键对话\n")
		for _, d := range scene.Dialogues {
			b.WriteString(fmt.Sprintf("- %s: \"%s\" (%s)\n", d.Speaker, d.Line, d.Context))
		}
	}

	b.WriteString("\n## 回答规则\n")
	b.WriteString("- 只提供你持有的数据，不编造\n")
	b.WriteString("- 回答要精确，引用事件编号\n")
	b.WriteString("- 不要替网文Agent写正文\n")

	return b.String()
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func extractEmotionFrom(thought string) string {
	emotions := []string{"愤怒", "恐惧", "冷静", "兴奋", "悲伤", "疑惑", "坚定", "犹豫", "惊喜", "警惕", "决然", "挣扎", "紧张", "狂傲", "自信"}
	for _, em := range emotions {
		if strings.Contains(thought, em) {
			return em
		}
	}
	return "平静"
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

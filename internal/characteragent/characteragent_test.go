package characteragent

import (
	"strings"
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
)

func TestDeriveProfileFromNode(t *testing.T) {
	graph := storybible.NewGraph()
	n := graph.AddNode(storybible.Node{
		Name: "叶凡",
		Kind: storybible.KindCharacter,
		Properties: map[string]interface{}{
			"角色定位": "主角",
			"性格标签": "坚毅, 果敢, 隐忍, 偏执",
			"说话风格": "言简意赅，不怒自威",
			"习惯动作": "摸刀柄, 拇指摩挲剑穗",
			"核心欲望": "守护身边人",
			"内心恐惧": "重蹈前世覆辙",
			"底线":   "不伤无辜, 不背叛朋友",
			"当前目标": "突破金丹期, 寻找失踪的师父",
		},
	})

	profile := DeriveProfileFromNode(n)
	if profile == nil {
		t.Fatal("DeriveProfileFromNode returned nil")
	}
	if profile.Name != "叶凡" {
		t.Fatalf("name mismatch: %s", profile.Name)
	}
	if profile.Role != "主角" {
		t.Fatalf("role mismatch: %s", profile.Role)
	}
	if len(profile.Personality) != 4 {
		t.Fatalf("expected 4 personality tags, got %d: %v", len(profile.Personality), profile.Personality)
	}
	if len(profile.RedLines) != 2 {
		t.Fatalf("expected 2 red lines, got %d: %v", len(profile.RedLines), profile.RedLines)
	}
	if len(profile.Goals) != 2 {
		t.Fatalf("expected 2 goals, got %d: %v", len(profile.Goals), profile.Goals)
	}
}

func TestDeriveProfileFromNilNode(t *testing.T) {
	if DeriveProfileFromNode(nil) != nil {
		t.Fatal("should return nil for nil node")
	}
}

func TestDeriveProfileFromNonCharacter(t *testing.T) {
	graph := storybible.NewGraph()
	n := graph.AddNode(storybible.Node{Name: "青云宗", Kind: storybible.KindFaction})
	if DeriveProfileFromNode(n) != nil {
		t.Fatal("should return nil for non-character node")
	}
}

func TestRenderSystemPrompt(t *testing.T) {
	p := &CharacterProfile{
		Name:        "叶凡",
		Role:        "主角",
		Personality: []string{"坚毅", "果敢", "隐忍", "偏执"},
		SpeechStyle: "言简意赅，不怒自威",
		Habits:      []string{"摸刀柄"},
		CoreDesire:  "守护身边人",
		InnerFear:   "重蹈覆辙",
		RedLines:    []string{"不伤无辜"},
		Goals:       []string{"突破金丹期"},
	}

	prompt := p.RenderSystemPrompt()
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	// Should contain key elements
	checks := []string{
		"叶凡",
		"主角",
		"坚毅",
		"言简意赅",
		"摸刀柄",
		"守护身边人",
		"重蹈覆辙",
		"不伤无辜",
		"突破金丹期",
		"YAML",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt should contain %q", c)
		}
	}
}

func TestMemorySummaryRender(t *testing.T) {
	m := &MemorySummary{
		CharacterID:  "char_1",
		RecentEvents: []string{"在第12章击败了妖兽", "在第13章进入青云宗", "在第14章与林婉儿结盟"},
		ChapterRange: "第12-14章",
	}
	rendered := m.Render()
	if rendered == "" {
		t.Fatal("rendered memory should not be empty")
	}
	if !strings.Contains(rendered, "第12-14章") {
		t.Error("should contain chapter range")
	}
	if !strings.Contains(rendered, "击败了妖兽") {
		t.Error("should contain event summary")
	}
}

func TestTriggerEventRender(t *testing.T) {
	te := TriggerEvent{
		EventType:     "突袭",
		Description:   "魔教突然袭击青云宗山门",
		InvolvedChars: []string{"叶凡", "林婉儿", "魔教长老"},
		Location:      "青云宗山门",
		ChapterNum:    15,
	}
	rendered := te.Render()
	if rendered == "" {
		t.Fatal("rendered event should not be empty")
	}
	if !strings.Contains(rendered, "突袭") {
		t.Error("should contain event type")
	}
	if !strings.Contains(rendered, "魔教") {
		t.Error("should contain description")
	}
	if !strings.Contains(rendered, "叶凡") {
		t.Error("should contain involved character")
	}
}

func TestReactionRender(t *testing.T) {
	r := &Reaction{
		CharacterID:   "char_1",
		CharacterName: "叶凡",
		InternalThought: "魔教突然来袭，此事必有蹊跷。先护住林婉儿，再探查真相。",
		Actions: []ActionCandidate{
			{Action: "先护住林婉儿", Priority: "高", TriggerCond: "魔教攻入山门"},
			{Action: "探查魔教动机", Priority: "中", TriggerCond: "击退第一波攻击后"},
		},
		RelationshipChanges: []RelChange{
			{TargetName: "林婉儿", Change: "关系更紧密", Reason: "共同御敌"},
		},
	}
	rendered := r.Render()
	if rendered == "" {
		t.Fatal("reaction render should not be empty")
	}
	if !strings.Contains(rendered, "叶凡") {
		t.Error("should contain character name")
	}
	if !strings.Contains(rendered, "护住林婉儿") {
		t.Error("should contain action")
	}
	if !strings.Contains(rendered, "关系更紧密") {
		t.Error("should contain relationship change")
	}
}

func TestBuildPrompt(t *testing.T) {
	graph := storybible.NewGraph()
	charA := graph.AddNode(storybible.Node{Name: "叶凡", Kind: storybible.KindCharacter,
		Properties: map[string]interface{}{
			"角色定位": "主角",
			"性格标签": "坚毅, 果敢",
			"当前实力": "金丹期",
		}})
	charB := graph.AddNode(storybible.Node{Name: "林婉儿", Kind: storybible.KindCharacter,
		Properties: map[string]interface{}{"角色定位": "女主"}})
	graph.AddEdge(storybible.Edge{From: charA.ID, To: charB.ID, Kind: storybible.RelAlly})

	snap := graph.SnapshotForCharacters([]string{charA.ID, charB.ID})
	profile := DeriveProfileFromNode(charA)

	agent := &Agent{
		Profile:   profile,
		GraphSnap: snap,
		Memory: &MemorySummary{
			CharacterID:  charA.ID,
			RecentEvents: []string{"第14章与林婉儿结盟"},
			ChapterRange: "第14章",
		},
	}

	event := TriggerEvent{
		EventType:     "突袭",
		Description:   "魔教突袭青云宗",
		InvolvedChars: []string{"叶凡", "林婉儿"},
		Location:      "青云宗",
		ChapterNum:    15,
	}

	prompt := agent.BuildPrompt(event)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}

	// Verify key elements
	checks := []string{
		"叶凡", "主角", "坚毅", "果敢",     // profile
		"林婉儿", "ally",                  // snapshot
		"第14章", "结盟",                   // memory
		"突袭", "魔教", "青云宗",            // event
		"不能读其他角色的内心",               // final instruction
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBatchSpecBuildAllPrompts(t *testing.T) {
	graph := storybible.NewGraph()
	charA := graph.AddNode(storybible.Node{Name: "叶凡", Kind: storybible.KindCharacter,
		Properties: map[string]interface{}{"角色定位": "主角", "性格标签": "坚毅"}})
	charB := graph.AddNode(storybible.Node{Name: "林婉儿", Kind: storybible.KindCharacter,
		Properties: map[string]interface{}{"角色定位": "女主", "性格标签": "聪慧"}})
	graph.AddEdge(storybible.Edge{From: charA.ID, To: charB.ID, Kind: storybible.RelAlly})

	snap := graph.SnapshotForCharacters([]string{charA.ID, charB.ID})

	batch := &BatchSpec{
		Profiles: []*CharacterProfile{
			DeriveProfileFromNode(charA),
			DeriveProfileFromNode(charB),
		},
		GraphSnap: snap,
		Event: TriggerEvent{
			EventType:   "对话",
			Description: "两人在山门前相遇",
			ChapterNum:  1,
		},
	}

	prompts := batch.BuildAllPrompts()
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	for _, prompt := range prompts {
		if !strings.Contains(prompt, "不能读其他角色的内心") {
			t.Error("each prompt should include the mind-reading prohibition")
		}
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"坚毅, 果敢, 隐忍", 3},
		{"坚毅", 1},
		{"", 0},
		{"坚毅,  , 隐忍", 2}, // empty entry skipped
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != tt.expected {
			t.Errorf("splitCSV(%q) = %d items, want %d", tt.input, len(got), tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	s := "这是一个很长的字符串需要被截断测试"
	result := truncate(s, 5)
	if len([]rune(result)) > 5+1 { // +1 for …
		t.Fatalf("truncate should limit to maxLen+1, got %d runes: %s", len([]rune(result)), result)
	}
}

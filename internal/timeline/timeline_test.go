package timeline

import (
	"strings"
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/characteragent"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storybible"
)

func TestNewEngine(t *testing.T) {
	g := storybible.NewGraph()
	e := NewEngine(g, "char_protagonist")
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.protagonistID != "char_protagonist" {
		t.Fatalf("protagonistID mismatch: %s", e.protagonistID)
	}
}

func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		action   string
		expected string
	}{
		{"攻击魔教长老", "攻击"},
		{"偷袭后方", "攻击"},
		{"杀死敌人", "攻击"},
		{"保护林婉儿", "保护"},
		{"护住山门", "保护"},
		{"掩护撤退", "保护"},
		{"离开现场", "离开"},
		{"逃跑", "离开"},
		{"揭示真相", "揭示"},
		{"隐藏身份", "隐藏"},
		{"劝说盟友", "劝说"},
		{"谈判条件", "劝说"},
		{"随便走走", "其他"},
	}
	for _, tt := range tests {
		got := classifyIntent(tt.action)
		if got != tt.expected {
			t.Errorf("classifyIntent(%q) = %q, want %q", tt.action, got, tt.expected)
		}
	}
}

func TestAreOpposed(t *testing.T) {
	if !areOpposed("攻击", "保护") {
		t.Error("攻击 vs 保护 should be opposed")
	}
	if !areOpposed("保护", "攻击") {
		t.Error("保护 vs 攻击 should be opposed")
	}
	if !areOpposed("揭示", "隐藏") {
		t.Error("揭示 vs 隐藏 should be opposed")
	}
	if areOpposed("攻击", "攻击") {
		t.Error("攻击 vs 攻击 should NOT be opposed")
	}
	if areOpposed("攻击", "离开") {
		t.Error("攻击 vs 离开 should NOT be opposed")
	}
}

func TestDetectConflicts(t *testing.T) {
	g := storybible.NewGraph()
	e := NewEngine(g, "char_protagonist")

	reactions := []characteragent.Reaction{
		{
			CharacterID:   "char_1",
			CharacterName: "叶凡",
			Actions: []characteragent.ActionCandidate{
				{Action: "攻击魔教长老", Priority: "高"},
			},
		},
		{
			CharacterID:   "char_2",
			CharacterName: "林婉儿",
			Actions: []characteragent.ActionCandidate{
				{Action: "保护魔教长老", Priority: "高"},
			},
		},
	}

	conflicts := e.detectConflicts(reactions, nil)
	if len(conflicts) == 0 {
		t.Fatal("should detect conflict between attack and protect")
	}
	t.Logf("detected conflict: %+v", conflicts[0])
}

func TestDetectConflictsNoOpposition(t *testing.T) {
	g := storybible.NewGraph()
	e := NewEngine(g, "char_protagonist")

	reactions := []characteragent.Reaction{
		{
			CharacterID:   "char_1",
			CharacterName: "叶凡",
			Actions: []characteragent.ActionCandidate{
				{Action: "攻击魔教长老", Priority: "高"},
			},
		},
		{
			CharacterID:   "char_2",
			CharacterName: "林婉儿",
			Actions: []characteragent.ActionCandidate{
				{Action: "离开现场", Priority: "高"},
			},
		},
	}

	conflicts := e.detectConflicts(reactions, nil)
	if len(conflicts) != 0 {
		t.Fatalf("should not detect conflict, got %d", len(conflicts))
	}
}

func TestResolveConflicts(t *testing.T) {
	g := storybible.NewGraph()
	e := NewEngine(g, "char_protagonist")

	conflicts := []ConflictResolution{
		{Between: []string{"叶凡", "林婉儿"}, ConflictDesc: "叶凡倾向攻击，林婉儿倾向保护"},
	}
	resolved := e.resolveConflicts(conflicts, "char_protagonist")
	if len(resolved) != 1 {
		t.Fatal("should resolve 1 conflict")
	}
	if resolved[0].Resolution == "" {
		t.Fatal("resolution should not be empty")
	}
}

func TestSynthesize(t *testing.T) {
	g := storybible.NewGraph()
	charA := g.AddNode(storybible.Node{Name: "叶凡", Kind: storybible.KindCharacter})
	charB := g.AddNode(storybible.Node{Name: "林婉儿", Kind: storybible.KindCharacter})
	charC := g.AddNode(storybible.Node{Name: "魔教长老", Kind: storybible.KindCharacter})
	g.AddEdge(storybible.Edge{From: charA.ID, To: charB.ID, Kind: storybible.RelAlly})

	e := NewEngine(g, charA.ID)

	profiles := map[string]*characteragent.CharacterProfile{
		charA.ID: {NodeID: charA.ID, Name: "叶凡", Role: "主角"},
		charB.ID: {NodeID: charB.ID, Name: "林婉儿", Role: "女主"},
		charC.ID: {NodeID: charC.ID, Name: "魔教长老", Role: "反派"},
	}

	input := SynthesisInput{
		ChapterNum: 15,
		Reactions: []characteragent.Reaction{
			{
				CharacterID:    charA.ID,
				CharacterName:  "叶凡",
				InternalThought: "魔教来犯，必须果断出击。",
				Actions: []characteragent.ActionCandidate{
					{Action: "攻击魔教长老", Priority: "高"},
					{Action: "探查魔教动机", Priority: "中"},
				},
				RelationshipChanges: []characteragent.RelChange{
					{TargetName: "林婉儿", Change: "关系更紧密", Reason: "共同御敌"},
				},
			},
			{
				CharacterID:    charB.ID,
				CharacterName:  "林婉儿",
				InternalThought: "配合叶凡行动，侧面牵制敌人。",
				Actions: []characteragent.ActionCandidate{
					{Action: "侧面牵制魔教弟子", Priority: "高"},
				},
				RelationshipChanges: []characteragent.RelChange{
					{TargetName: "叶凡", Change: "信任加深", Reason: "并肩作战"},
				},
			},
		},
		Profiles:      profiles,
		ProtagonistID: charA.ID,
		GraphSnap:     g.SnapshotForCharacters([]string{charA.ID, charB.ID, charC.ID}),
	}

	out := e.Synthesize(input)
	if out == nil {
		t.Fatal("Synthesize returned nil")
	}
	if out.Metadata.NumReactions != 2 {
		t.Fatalf("expected 2 reactions, got %d", out.Metadata.NumReactions)
	}
	// Duration may round to 0 for very fast synthesis — that's fine on fast hardware.
	_ = out.Metadata.Duration
	if len(out.GraphBatch.Instructions) < 1 {
		t.Fatalf("should produce at least 1 graph update instruction, got %d", len(out.GraphBatch.Instructions))
	}
	if out.GraphBatch.Chapter != 15 {
		t.Fatalf("chapter should be 15, got %d", out.GraphBatch.Chapter)
	}

	t.Logf("synthesis: %d scene beats, %d conflicts, %d graph mutations, duration=%v",
		len(out.Narrative), out.Metadata.NumConflicts, out.Metadata.NumGraphMutations, out.Metadata.Duration)

	for _, beat := range out.Narrative {
		t.Logf("  beat: [%s] %s — %v", beat.Phase, beat.Description, beat.Actions)
	}
}

func TestSynthesizeEmptyReactions(t *testing.T) {
	g := storybible.NewGraph()
	e := NewEngine(g, "char_1")

	input := SynthesisInput{
		ChapterNum:    1,
		Reactions:     nil,
		ProtagonistID: "char_1",
	}

	out := e.Synthesize(input)
	if out == nil {
		t.Fatal("Synthesize should not return nil for empty input")
	}
	if out.Metadata.NumReactions != 0 {
		t.Fatal("should have 0 reactions")
	}
}

func TestMapRelChangeToKind(t *testing.T) {
	tests := []struct {
		change   string
		expected storybible.RelKind
	}{
		{"关系敌对", storybible.RelEnemy},
		{"背叛了信任", storybible.RelEnemy},
		{"关系更紧密", storybible.RelAlly},
		{"建立同盟", storybible.RelAlly},
		{"拜师学艺", storybible.RelMentor},
		{"产生爱慕", storybible.RelLover},
		{"成为竞争对手", storybible.RelRival},
		{"其他变化", storybible.RelCustom},
	}
	for _, tt := range tests {
		got := mapRelChangeToKind(tt.change)
		if got != tt.expected {
			t.Errorf("mapRelChangeToKind(%q) = %s, want %s", tt.change, got, tt.expected)
		}
	}
}

func TestExtractEmotion(t *testing.T) {
	tests := []struct {
		thought  string
		expected string
	}{
		{"我感到非常愤怒", "愤怒"},
		{"心中充满恐惧", "恐惧"},
		{"必须保持冷静", "冷静"},
		{"今天天气不错", "平静"},
	}
	for _, tt := range tests {
		got := extractEmotion(tt.thought)
		if got != tt.expected {
			t.Errorf("extractEmotion(%q) = %q, want %q", tt.thought, got, tt.expected)
		}
	}
}

func TestBuildLLMPrompt(t *testing.T) {
	input := SynthesisInput{
		ChapterNum:    10,
		ProtagonistID: "char_1",
	}
	reactions := []characteragent.Reaction{
		{
			CharacterID:    "char_1",
			CharacterName:  "叶凡",
			InternalThought: "必须冷静应对。",
			Actions: []characteragent.ActionCandidate{
				{Action: "出手攻击", Priority: "高"},
			},
		},
	}

	prompt := BuildLLMPrompt(input, reactions)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}

	checks := []string{
		"时间线合成Agent",
		"铺垫",
		"冲突升级",
		"高潮/爽点",
		"悬念收尾",
		"第10章",
		"叶凡",
		"graph_updates",
		"conflict_resolutions",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestResolveCharacterID(t *testing.T) {
	profiles := map[string]*characteragent.CharacterProfile{
		"id_a": {NodeID: "id_a", Name: "叶凡"},
		"id_b": {NodeID: "id_b", Name: "林婉儿"},
	}

	got := resolveCharacterID("叶凡", profiles)
	if got != "id_a" {
		t.Fatalf("expected id_a, got %s", got)
	}

	// Unknown character
	got = resolveCharacterID("未知角色", profiles)
	if got != "未知角色" {
		t.Fatalf("should fallback to name, got %s", got)
	}
}

func TestAssignPhases(t *testing.T) {
	actions := []scorableAction{
		{charName: "叶凡", action: "行动1", priority: 3, emotion: "坚定"},
		{charName: "叶凡", action: "行动2", priority: 3, emotion: "坚定"},
		{charName: "林婉儿", action: "行动3", priority: 2, emotion: "警惕"},
		{charName: "魔教长老", action: "行动4", priority: 2, emotion: "愤怒"},
	}

	result := assignPhases(actions, 4)
	if len(result) != 4 {
		t.Fatalf("expected 4 phases, got %d", len(result))
	}
	for _, pa := range result {
		if pa.phaseIdx < 0 || pa.phaseIdx >= 4 {
			t.Errorf("invalid phase index: %d", pa.phaseIdx)
		}
		t.Logf("phase %d (%s): %s actions=%v", pa.phaseIdx, WebNovelRhythm[pa.phaseIdx], pa.charName, pa.actions)
	}
}

func TestAssignPhasesFewActions(t *testing.T) {
	actions := []scorableAction{
		{charName: "叶凡", action: "唯一行动", priority: 3, emotion: "坚定"},
	}

	result := assignPhases(actions, 4)
	if len(result) != 4 {
		t.Fatalf("expected 4 phases, got %d", len(result))
	}
	// First phase should have the action
	if len(result[0].actions) != 1 || result[0].actions[0] != "唯一行动" {
		t.Errorf("first phase should contain the action, got %v", result[0].actions)
	}
}

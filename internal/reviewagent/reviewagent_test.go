package reviewagent

import (
	"strings"
	"testing"
)

func TestDefaultAIPatterns(t *testing.T) {
	patterns := DefaultAIPatterns()
	if len(patterns) < 5 {
		t.Fatalf("expected at least 5 patterns, got %d", len(patterns))
	}
	for _, p := range patterns {
		if p.Name == "" || p.Description == "" || p.Severity == "" {
			t.Errorf("pattern has empty fields: %+v", p)
		}
		if len(p.Examples) < 1 {
			t.Errorf("pattern %q has no examples", p.Name)
		}
	}
}

func TestBuildExpandPrompt(t *testing.T) {
	in := Input{
		Mode:        ModeExpand,
		Content:     "叶凡推开山门，走了进去。里面空无一人。",
		ChapterNum:  5,
		Genre:       "玄幻",
		ExpandRatio: 1.5,
	}
	prompt := BuildExpandPrompt(in)
	if prompt == "" {
		t.Fatal("expand prompt should not be empty")
	}
	checks := []string{
		"扩写", "叶凡",
		"感官细节", "微表情", "环境描写",
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBuildRewriteAnalyzePrompt(t *testing.T) {
	in := Input{
		Mode:         ModeRewrite,
		Content:      "叶凡看着远处的山，心中感慨万千。",
		ChapterNum:   3,
		Genre:        "玄幻",
		RewriteBrief: "改成第一人称，更有代入感",
	}
	prompt := BuildRewriteAnalyzePrompt(in)
	if prompt == "" {
		t.Fatal("rewrite analyze prompt should not be empty")
	}
	for _, c := range []string{"改写", "需求分析", "第一人称", "questions"} {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestBuildRewriteExecutePrompt(t *testing.T) {
	in := Input{
		Mode:    ModeRewrite,
		Content: "测试正文。",
	}
	answers := map[string]string{
		"q1": "第一人称",
		"q2": "简洁风格",
	}
	prompt := BuildRewriteExecutePrompt(in, answers)
	if prompt == "" {
		t.Fatal("rewrite execute prompt should not be empty")
	}
	if !strings.Contains(prompt, "第一人称") {
		t.Error("prompt should include user answers")
	}
	if !strings.Contains(prompt, "简洁风格") {
		t.Error("prompt should include user answers")
	}
}

func TestBuildDeAIPrompt(t *testing.T) {
	in := Input{
		Mode:       ModeDeAI,
		Content:    "与此同时，他的心中涌起了一股难以言喻的情绪。他既感到愤怒，又感到无奈。",
		ChapterNum: 1,
		Genre:      "都市",
	}
	prompt := BuildDeAIPrompt(in)
	if prompt == "" {
		t.Fatal("deAI prompt should not be empty")
	}
	for _, c := range []string{
		"去AI化", "万能过渡句", "空洞情绪", "对称句式",
		"修复原则", "detected_patterns", "changes",
	} {
		if !strings.Contains(prompt, c) {
			t.Errorf("prompt missing %q", c)
		}
	}
}

func TestRewriteDialogue(t *testing.T) {
	in := Input{
		Mode:    ModeRewrite,
		Content: "测试正文",
	}
	d := NewRewriteDialogue(in)

	if d.Stage != "analyze" {
		t.Fatalf("initial stage should be analyze, got %s", d.Stage)
	}
	if d.IsComplete() {
		t.Fatal("should not be complete with no questions")
	}

	// Set questions
	d.SetQuestions([]RewriteQuestion{
		{ID: "q1", Question: "选择叙事视角", Options: []string{"第一人称", "第三人称"}, Default: "第三人称"},
		{ID: "q2", Question: "选择风格", Options: []string{"简洁", "华丽", "白描"}, Default: "简洁"},
	})

	if d.Stage != "confirm" {
		t.Fatalf("stage should be confirm after setting questions, got %s", d.Stage)
	}
	if d.IsComplete() {
		t.Fatal("should not be complete with 2 unanswered questions")
	}

	// Record one answer
	done := d.RecordAnswer("q1", "第一人称")
	if done {
		t.Fatal("should not be done after 1/2 answers")
	}

	// Record second answer
	done = d.RecordAnswer("q2", "华丽")
	if !done {
		t.Fatal("should be done after 2/2 answers")
	}
	if !d.IsComplete() {
		t.Fatal("should be complete")
	}
}

func TestBuildConfirmPrompt(t *testing.T) {
	in := Input{Mode: ModeRewrite, Content: "test"}
	d := NewRewriteDialogue(in)
	d.SetQuestions([]RewriteQuestion{
		{ID: "q1", Question: "选择语气", Options: []string{"冷峻", "热血", "幽默"}, Default: "热血"},
	})
	prompt := d.BuildConfirmPrompt()
	if prompt == "" {
		t.Fatal("confirm prompt should not be empty")
	}
	if !strings.Contains(prompt, "改写确认") {
		t.Error("should contain 改写确认")
	}
	if !strings.Contains(prompt, "冷峻") {
		t.Error("should contain options")
	}
	// Default marker
	if !strings.Contains(prompt, "→") {
		t.Error("should show default marker")
	}
}

func TestAllModes(t *testing.T) {
	modes := []Mode{ModeExpand, ModeRewrite, ModeDeAI}
	for _, m := range modes {
		in := Input{Mode: m, Content: "测试正文", ChapterNum: 1, Genre: "玄幻", ExpandRatio: 1.3}
		var prompt string
		switch m {
		case ModeExpand:
			prompt = BuildExpandPrompt(in)
		case ModeRewrite:
			prompt = BuildRewriteAnalyzePrompt(in)
		case ModeDeAI:
			prompt = BuildDeAIPrompt(in)
		}
		if prompt == "" {
			t.Errorf("mode %q produced empty prompt", m)
		}
		if !strings.Contains(prompt, string(m)) && m != ModeDeAI {
			// DeAI mode doesn't literally contain "deai" in Chinese prompts
			if m == ModeExpand && !strings.Contains(prompt, "扩写") {
				t.Errorf("expand prompt should mention 扩写")
			}
		}
	}
}

func TestOutputStruct(t *testing.T) {
	o := Output{
		Mode:         ModeDeAI,
		OriginalLen:  1000,
		ResultLen:    950,
		ProcessedText: "处理后的文本",
		Changes: []Change{
			{Location: "第2段", Before: "原文", After: "修改", Reason: "去AI化", Pattern: "万能过渡句"},
		},
		DetectedPatterns: []PatternHit{
			{Pattern: "万能过渡句", Count: 3, Examples: []string{"例1"}, Severity: "high"},
		},
	}
	if o.Mode != ModeDeAI {
		t.Fatal("mode mismatch")
	}
	if len(o.Changes) != 1 {
		t.Fatal("should have 1 change")
	}
	if len(o.DetectedPatterns) != 1 {
		t.Fatal("should have 1 detected pattern")
	}
	if o.DetectedPatterns[0].Count != 3 {
		t.Fatalf("count mismatch: %d", o.DetectedPatterns[0].Count)
	}
}

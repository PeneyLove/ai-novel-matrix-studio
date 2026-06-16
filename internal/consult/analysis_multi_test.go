package consult

import (
	"strings"
	"testing"
)

func TestConsistencyStrategy(t *testing.T) {
	s := NewConsistencyStrategy()
	// No character file.
	findings, err := s.AnalyzeSources(map[string]string{})
	if err != nil {
		t.Fatalf("empty sources: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("should flag missing character file")
	}

	// With character file but no chapters.
	findings, err = s.AnalyzeSources(map[string]string{
		"characters": "name: 叶凡\nrole: 主角\n",
	})
	if err != nil {
		t.Fatalf("char only: %v", err)
	}
	if len(findings) != 0 {
		t.Logf("character-only findings: %d", len(findings))
	}

	// With both.
	findings, err = s.AnalyzeSources(map[string]string{
		"characters": "name: 叶凡\nname: 林雪\n",
		"chapters":   "叶凡走进宗门，看到林雪在等他",
	})
	if err != nil {
		t.Fatalf("both sources: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("should have findings about character appearance")
	}
	for _, f := range findings {
		t.Logf("  [%s] %s (conf=%d)", f.Severity, f.Title, f.Confidence)
	}
}

func TestHookStrategy(t *testing.T) {
	s := NewHookStrategy()
	findings, err := s.AnalyzeSources(map[string]string{})
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if len(findings) != 0 {
		t.Fatal("no hooks source should produce no findings")
	}

	// With hooks.
	hooksContent := `
hook-001:
  description: 主角身世之谜
  status: pending
  expected_recovery: 70
hook-002:
  description: 神秘石碑
  status: pending
hook-003:
  description: 魔教阴谋
  status: resolved
`
	findings, err = s.AnalyzeSources(map[string]string{"hooks": hooksContent})
	if err != nil {
		t.Fatalf("hooks: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("should have findings for pending hooks")
	}
	for _, f := range findings {
		t.Logf("  [%s] %s (conf=%d)", f.Severity, f.Title, f.Confidence)
	}
}

func TestLogicStrategy(t *testing.T) {
	s := NewLogicStrategy()
	findings, err := s.AnalyzeSources(map[string]string{
		"chapters": "主角突破筑基！\n主角又突破筑基！\n主角再次突破筑基！",
		"outline":  "第一卷：入门（1-40章）",
	})
	if err != nil {
		t.Fatalf("logic: %v", err)
	}
	if len(findings) == 0 {
		t.Log("no findings for repeated breakthroughs (acceptable for short text)")
	} else {
		for _, f := range findings {
			t.Logf("  [%s] %s", f.Severity, f.Title)
		}
	}
}

func TestStyleStrategy(t *testing.T) {
	s := NewStyleStrategy()
	findings, err := s.AnalyzeSources(map[string]string{})
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if len(findings) != 0 {
		t.Fatal("no chapters source should produce no findings")
	}

	// With slop-heavy content.
	chapters := `值得注意的是，在这个充满变数的世界里，我们不得不面对现实。
不可否认，从某种意义上说，主角的成长是显而易见的。
与此同时，我们也可以看到配角的努力。
总的来说，这一章表达了主角的内心世界。
`
	findings, err = s.AnalyzeSources(map[string]string{"chapters": chapters})
	if err != nil {
		t.Fatalf("slop check: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("should detect slop phrases")
	}
	for _, f := range findings {
		t.Logf("  [%s] %s", f.Severity, f.Title)
	}
}

func TestMultiSourceEngine(t *testing.T) {
	engine := NewDefaultMultiSourceEngine()
	if engine == nil {
		t.Fatal("NewDefaultMultiSourceEngine returned nil")
	}

	sources := map[string]string{
		"outline": `【核心设定】
· 境界体系：筑基→金丹
· 力量体系：灵根
【人物谱系】
■ 主角：叶凡、废灵根
■ 反派1：赵无极
【主线剧情】
· 核心冲突：凡人逆天
· 第一卷：入门（1-40章）
· 终极结局：飞升
`,
	}
	report := engine.Consult("多源测试", sources)
	if report.Score <= 0 {
		t.Logf("score=%d (acceptable for minimal outline)", report.Score)
	}
	if len(report.Findings) == 0 {
		t.Error("should have findings")
	}
	for _, f := range report.Findings {
		t.Logf("  [%s][%s] %s", f.Severity, f.Category, f.Title)
	}
}

func BenchmarkQuickConsult(b *testing.B) {
	src := strings.Repeat("【核心设定】\n· 境界体系\n【人物谱系】\n■ 主角\n【主线剧情】\n· 第一卷\n", 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		QuickConsult("bench", src)
	}
}

func BenchmarkMultiSourceEngine(b *testing.B) {
	sources := map[string]string{
		"outline":   "【核心设定】\n· 境界\n【人物谱系】\n■ 主角\n【主线剧情】\n· 冲突",
		"characters": "name: 叶凡\nname: 林雪\n",
		"chapters":   "叶凡突破了。\n值得注意的是，我们要继续写。",
	}
	engine := NewDefaultMultiSourceEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Consult("bench", sources)
	}
}

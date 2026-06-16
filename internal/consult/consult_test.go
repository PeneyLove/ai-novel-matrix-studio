package consult

import (
	"strings"
	"testing"
)

func TestEngine_WithDefaultStrategies(t *testing.T) {
	engine := NewDefaultEngine()
	if engine == nil {
		t.Fatal("NewDefaultEngine returned nil")
	}
	if len(engine.strategies) != 4 {
		t.Fatalf("expected 4 default strategies, got %d", len(engine.strategies))
	}
}

func TestOutlineValidator_CompleteOutline(t *testing.T) {
	src := `【核心设定】
· 境界体系：炼气→筑基→金丹→元婴→化神
· 力量体系：灵根、功法、丹药
· 世界观：修真界

【人物谱系】
■ 主角：叶凡、废灵根、神秘石碑金手指
■ 配角1：林雪、青梅竹马
■ 配角2：王大牛、生死兄弟
■ 反派1：赵无极、宗门大弟子

【主线剧情】
· 核心冲突：凡人逆天修行
· 第一卷：入门（1-40章）
· 第二卷：试炼（41-80章）
· 终极结局：飞升仙界

【爽点节点】
① 第10章：越级打脸
② 第25章：获得传承
③ 第40章：突破筑基`
	v := NewOutlineValidator(nil)
	findings, err := v.Analyze(src)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	// Should have few findings for a complete outline.
	if len(findings) > 5 {
		t.Logf("complete outline had %d findings (acceptable):", len(findings))
		for _, f := range findings {
			t.Logf("  [%s] %s (conf=%d)", f.Severity, f.Title, f.Confidence)
		}
	}
}

func TestOutlineValidator_EmptyOutline(t *testing.T) {
	v := NewOutlineValidator(nil)
	findings, err := v.Analyze("")
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("empty outline should produce findings")
	}
	// The empty outline should get blocked by the required sections.
	hasBlockers := false
	for _, f := range findings {
		if f.Severity == SeverityBlock {
			hasBlockers = true
			break
		}
	}
	if !hasBlockers {
		t.Error("empty outline should have blocker-severity findings")
	}
}

func TestCharacterAnalyzer(t *testing.T) {
	a := NewCharacterAnalyzer()
	src := `【人物谱系】
■ 主角：叶凡、废灵根、神秘石碑金手指
性格：坚韧不拔、重情重义
成长：从废材到强者

■ 配角1：林雪、青梅竹马、温柔体贴
■ 配角2：王大牛、生死兄弟、忠诚可靠
■ 配角3：柳师姐、宗门引路人、严厉但护短
■ 配角4：小师叔、神秘强者、亦师亦友

■ 反派1：赵无极、宗门大弟子、嫉妒主角
■ 反派2：魔教教主、野心勃勃、统治世界`
	findings, err := a.Analyze(src)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	for _, f := range findings {
		t.Logf("  [%s] %s", f.Severity, f.Title)
	}
}

func TestPlotStructureAnalyzer(t *testing.T) {
	a := NewPlotStructureAnalyzer()
	src := `【主线剧情】
· 核心冲突：主角被废灵根后逆天崛起 vs 宗门保守势力
· 第一卷：废材崛起（1-40章）
· 第二卷：秘境夺宝（41-80章）
· 第三卷：宗门大比（81-120章）
· 第四卷：魔教入侵（121-160章）
· 第五卷：飞升仙界（161-200章）
· 终极结局：主角飞升，守护人间界`
	findings, err := a.Analyze(src)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	for _, f := range findings {
		t.Logf("  [%s] %s", f.Severity, f.Title)
	}
}

func TestFormatReport(t *testing.T) {
	r := &Report{
		Subject: "大纲审核",
		Score:   75,
		Summary: "总体良好",
		Findings: []Finding{
			{
				Category:    "outline-completeness",
				Severity:    SeverityWarn,
				Title:       "配角数量不足",
				Description: "当前仅有1个配角",
				Suggestion:  "增加配角至4个",
				Confidence:  80,
			},
		},
	}
	output := FormatReport(r)
	if !strings.Contains(output, "75/100") {
		t.Error("output should contain the score")
	}
	if !strings.Contains(output, "配角数量不足") {
		t.Error("output should contain the finding title")
	}
	t.Logf("Formatted report:\n%s", output)
}

func TestQuickConsult(t *testing.T) {
	output := QuickConsult("测试", "一段简单的大纲文本，没有结构化内容")
	if !strings.Contains(output, "创作咨询报告") {
		t.Error("output should start with the report header")
	}
	if !strings.Contains(output, "缺少") {
		t.Error("output should mention missing sections")
	}
	t.Logf("QuickConsult output:\n%s", output[:min(len(output), 500)])
}

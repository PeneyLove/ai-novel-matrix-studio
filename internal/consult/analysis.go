package consult

import (
	"regexp"
	"strings"
)

// CharacterAnalyzer checks character-related aspects of an outline: whether
// main characters have sufficient detail, whether supporting characters have
// clear roles, and whether antagonists have credible motivations.
type CharacterAnalyzer struct {
	headingPattern *regexp.Regexp
}

// NewCharacterAnalyzer creates a CharacterAnalyzer.
func NewCharacterAnalyzer() *CharacterAnalyzer {
	return &CharacterAnalyzer{
		headingPattern: regexp.MustCompile(`(?m)^【.+?】`),
	}
}

// Name returns "character-consistency".
func (c *CharacterAnalyzer) Name() string { return "character-consistency" }

// Description returns a description of this strategy.
func (c *CharacterAnalyzer) Description() string {
	return "检查人物设定完整性：主角弧光、配角功能定位、反派动机可信度"
}

// Analyze runs character checks against src.
func (c *CharacterAnalyzer) Analyze(src string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(src, "\n")

	// Find the character section.
	charSection := extractSectionAfterHeading(lines, "人物谱系")
	if charSection == "" {
		findings = append(findings, Finding{
			Category:    c.Name(),
			Severity:    SeverityWarn,
			Title:       "未找到人物谱系模块",
			Description: "大纲中缺少「人物谱系」模块，无法进行角色分析",
			Suggestion:  "添加人物谱系模块，包含主角、配角和反派的详细设定",
			Confidence:  90,
		})
		return findings, nil
	}

	// Check for protagonist.
	if !hasCharacterType(charSection, "主角") {
		findings = append(findings, Finding{
			Category:    c.Name(),
			Severity:    SeverityBlock,
			Title:       "缺少主角设定",
			Description: "人物谱系中未找到主角（主角）的完整描述",
			Suggestion:  "添加主角设定：全名、身世、金手指、性格标签（≥3个）和成长弧线",
			Confidence:  95,
		})
	} else {
		// Check protagonist detail level.
		protagonistLines := extractCharacterLines(charSection, "主角")
		if len(protagonistLines) < 3 {
			findings = append(findings, Finding{
				Category:    c.Name(),
				Severity:    SeverityWarn,
				Title:       "主角设定过于简略",
				Description: "主角描述仅" + formatInt(len(protagonistLines)) + "行，缺乏细节",
				Suggestion:  "扩展主角设定：身世背景、金手指类型、性格特点（≥3个标签）、成长弧线阶段",
				Confidence:  70,
			})
		}
	}

	// Check for supporting characters.
	supportingCount := countCharacterEntries(charSection, "配角")
	if supportingCount < 2 {
		findings = append(findings, Finding{
			Category:    c.Name(),
			Severity:    SeverityWarn,
			Title:       "配角数量不足",
			Description: "当前仅有" + formatInt(supportingCount) + "个配角，建议至少4个",
			Suggestion:  "增加配角至4个以上，确保每个配角有独立的剧情功能和人物弧光",
			Confidence:  80,
		})
	}

	// Check for antagonists.
	antagCount := countCharacterEntries(charSection, "反派")
	if antagCount < 1 {
		findings = append(findings, Finding{
			Category:    c.Name(),
			Severity:    SeverityWarn,
			Title:       "缺少反派设定",
			Description: "人物谱系中未找到反派角色",
			Suggestion:  "添加至少1个主要反派：动机、势力、与主角的冲突层次、压迫感设计",
			Confidence:  85,
		})
	} else if antagCount < 2 {
		findings = append(findings, Finding{
			Category:    c.Name(),
			Severity:    SeverityInfo,
			Title:       "反派数量偏少",
			Description: "仅有1个反派，故事后期可能缺乏冲突升级空间",
			Suggestion:  "考虑增加2-3个反派，形成层次分明的压迫体系（前期/中期/终极反派）",
			Confidence:  65,
		})
	}

	return findings, nil
}

// PlotStructureAnalyzer checks plot structure: conflict, volume division,
// climax distribution, and ending completeness.
type PlotStructureAnalyzer struct{}

// NewPlotStructureAnalyzer creates a PlotStructureAnalyzer.
func NewPlotStructureAnalyzer() *PlotStructureAnalyzer {
	return &PlotStructureAnalyzer{}
}

// Name returns "plot-structure".
func (p *PlotStructureAnalyzer) Name() string { return "plot-structure" }

// Description returns a description.
func (p *PlotStructureAnalyzer) Description() string {
	return "检查剧情结构：核心冲突、分卷规划、高潮分布、结局完整性"
}

// Analyze runs plot structure checks.
func (p *PlotStructureAnalyzer) Analyze(src string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(src, "\n")
	plotSection := extractSectionAfterHeading(lines, "主线剧情")

	if plotSection == "" {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityBlock,
			Title:       "缺少主线剧情模块",
			Description: "大纲中缺少「主线剧情」模块，无法分析剧情结构",
			Suggestion:  "添加主线剧情模块：核心冲突、分卷剧情、终极结局",
			Confidence:  95,
		})
		return findings, nil
	}

	// Check for core conflict.
	if !strings.Contains(plotSection, "冲突") && !strings.Contains(plotSection, "核心") {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityBlock,
			Title:       "缺少核心冲突",
			Description: "主线剧情中未明确核心冲突",
			Suggestion:  "用一句话概括核心冲突：主角 vs 什么样的势力/命运/内心，冲突如何贯穿全书",
			Confidence:  90,
		})
	}

	// Count volume markers.
	volumeCount := countVolumeMarkers(plotSection)
	if volumeCount == 0 {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityWarn,
			Title:       "缺少分卷规划",
			Description: "主线剧情中没有明确的分卷（卷/部），无法判断节奏分布",
			Suggestion:  "将主线拆分为5-8卷，每卷有独立的起承转合",
			Confidence:  85,
		})
	} else if volumeCount < 3 {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityWarn,
			Title:       "分卷数量偏少",
			Description: "仅有" + formatInt(volumeCount) + "个分卷，难以支撑长篇小说节奏",
			Suggestion:  "建议分卷数量在5-8卷，每卷40-60章",
			Confidence:  70,
		})
	}

	// Check for ending.
	if !hasEnding(plotSection) {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityWarn,
			Title:       "缺少结局设定",
			Description: "主线剧情中未明确终极结局",
			Suggestion:  "添加结局设定：主角最终状态、世界变化、感情线结局、主题升华",
			Confidence:  75,
		})
	}

	return findings, nil
}

// PacingAnalyzer checks outline pacing heuristics: high-point density,
// hook coverage, and escalation structure.
type PacingAnalyzer struct{}

// NewPacingAnalyzer creates a PacingAnalyzer.
func NewPacingAnalyzer() *PacingAnalyzer {
	return &PacingAnalyzer{}
}

// Name returns "pacing-health".
func (p *PacingAnalyzer) Name() string { return "pacing-health" }

// Description returns a description.
func (p *PacingAnalyzer) Description() string {
	return "检查节奏健康度：爽点密度、钩子分布、升级节奏"
}

// Analyze runs pacing checks.
func (p *PacingAnalyzer) Analyze(src string) ([]Finding, error) {
	var findings []Finding
	hpSection := extractSectionAfterHeading(strings.Split(src, "\n"), "爽点节点")

	if hpSection == "" {
		findings = append(findings, Finding{
			Category:    p.Name(),
			Severity:    SeverityInfo,
			Title:       "缺少爽点节点规划",
			Description: "大纲中未单独列出爽点节点，可能缺乏高潮节奏的把控",
			Suggestion:  "建议添加爽点节点模块：每卷标记3个核心爽点的类型和大致章节位置",
			Confidence:  70,
		})
	} else {
		hpCount := countHighPoints(hpSection)
		if hpCount < 3 {
			findings = append(findings, Finding{
				Category:    p.Name(),
				Severity:    SeverityWarn,
				Title:       "爽点数量偏少",
				Description: "仅列出" + formatInt(hpCount) + "个爽点节点",
				Suggestion:  "建议每卷安排3个以上爽点，类型多样（升级/打脸/逆袭/夺宝/突破/揭秘/扬名）",
				Confidence:  70,
			})
		}
	}

	return findings, nil
}

// --- shared helpers ---

// extractSectionAfterHeading returns the content after a heading in lines.
func extractSectionAfterHeading(lines []string, heading string) string {
	start := -1
	pat := regexp.MustCompile(`【` + regexp.QuoteMeta(heading) + `.*?】`)
	for i, line := range lines {
		if pat.MatchString(line) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	for i := start + 1; i < len(lines); i++ {
		if matched, _ := regexp.MatchString(`【.+?】`, lines[i]); matched {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
	}
	return b.String()
}

func hasCharacterType(section, charType string) bool {
	lower := strings.ToLower(section)
	return strings.Contains(lower, strings.ToLower(charType))
}

func extractCharacterLines(section, charType string) []string {
	var result []string
	lines := strings.Split(section, "\n")
	inTarget := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, charType) && (strings.HasPrefix(trimmed, "■") || strings.HasPrefix(trimmed, "·") || strings.HasPrefix(trimmed, "-")) {
			inTarget = true
			result = append(result, trimmed)
			continue
		}
		if inTarget {
			if strings.HasPrefix(trimmed, "■") || strings.HasPrefix(trimmed, "【") {
				break
			}
			if trimmed != "" {
				result = append(result, trimmed)
			} else {
				break
			}
		}
	}
	return result
}

func countCharacterEntries(section, charType string) int {
	count := 0
	lines := strings.Split(section, "\n")
	pat := regexp.MustCompile(charType + `\d*`)
	for _, line := range lines {
		if pat.MatchString(line) && (strings.HasPrefix(strings.TrimSpace(line), "■") || strings.HasPrefix(strings.TrimSpace(line), "-")) {
			count++
		}
	}
	return count
}

func countVolumeMarkers(section string) int {
	count := 0
	lines := strings.Split(section, "\n")
	volPat := regexp.MustCompile(`(?:第\s*[一二三四五六七八九十百千\d]+\s*卷|卷\s*[一二三四五六七八九十\d])`)
	for _, line := range lines {
		if volPat.MatchString(line) {
			count++
		}
	}
	return count
}

func hasEnding(section string) bool {
	lower := strings.ToLower(section)
	return strings.Contains(lower, "结局") || strings.Contains(lower, "结尾") || strings.Contains(lower, "终章")
}

func countHighPoints(section string) int {
	count := 0
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "①") || strings.HasPrefix(trimmed, "②") || strings.HasPrefix(trimmed, "③") ||
			strings.HasPrefix(trimmed, "④") || strings.HasPrefix(trimmed, "⑤") {
			count++
		}
		// Also count numbered patterns like "1." or "1)"
		if matched, _ := regexp.MatchString(`^\d+[\.\)]`, trimmed); matched && len(trimmed) < 60 {
			count++
		}
	}
	return count
}

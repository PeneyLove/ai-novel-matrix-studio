package consult

import (
	"fmt"
	"regexp"
	"strings"
)

// --- Outline-specific types ---

// OutlineSection identifies a required section in a novel outline.
type OutlineSection struct {
	Name     string // e.g. "核心设定", "人物谱系", "主线剧情"
	Keywords []string // keywords that must appear in the section
	Required bool   // true = blocking if missing
	Tip      string // suggestion when missing
}

// DefaultOutlineSections returns the standard set of sections expected in a
// novel outline, covering the common genre requirements.
func DefaultOutlineSections() []OutlineSection {
	return []OutlineSection{
		{
			Name:     "核心设定",
			Keywords: []string{"境界", "力量", "世界观", "世界"},
			Required: true,
			Tip:      "添加核心设定：境界体系、力量体系、世界观地图",
		},
		{
			Name:     "人物谱系",
			Keywords: []string{"主角", "配角", "反派"},
			Required: true,
			Tip:      "添加人物谱系：主角（身世+金手指+性格+成长弧线）、≥4配角、≥3反派",
		},
		{
			Name:     "主线剧情",
			Keywords: []string{"冲突", "卷", "剧情", "结局"},
			Required: true,
			Tip:      "添加主线剧情：核心冲突→5-8卷分卷剧情→终极结局",
		},
		{
			Name:     "爽点节点",
			Keywords: []string{"爽点", "高潮"},
			Required: false,
			Tip:      "建议添加爽点节点：每卷标记3个核心爽点（升级/打脸/逆袭/夺宝等）",
		},
		{
			Name:     "感情线",
			Keywords: []string{"感情", "CP", "爱情"},
			Required: false,
			Tip:      "考虑添加感情线（可选纯剧情流无感情线）",
		},
	}
}

// OutlineValidator checks a novel outline text for completeness, structure, and
// common issues. It implements the Strategy interface.
type OutlineValidator struct {
	sections        []OutlineSection
	headingPattern  *regexp.Regexp
	bulletPattern   *regexp.Regexp
}

// NewOutlineValidator creates a validator with the given required sections.
// If sections is nil, DefaultOutlineSections is used.
func NewOutlineValidator(sections []OutlineSection) *OutlineValidator {
	if sections == nil {
		sections = DefaultOutlineSections()
	}
	return &OutlineValidator{
		sections:       sections,
		headingPattern: regexp.MustCompile(`(?m)^【(.+?)】`),
		bulletPattern:  regexp.MustCompile(`(?m)^[·\-*\d+.]`),
	}
}

// Name returns "outline-completeness".
func (v *OutlineValidator) Name() string { return "outline-completeness" }

// Description returns a description of this strategy.
func (v *OutlineValidator) Description() string {
	return "检查大纲完整性：确认包含核心设定、人物谱系、主线剧情等必要模块"
}

// Analyze runs the outline completeness check against src.
func (v *OutlineValidator) Analyze(src string) ([]Finding, error) {
	var findings []Finding

	sections := extractSections(src, v.headingPattern)
	lines := strings.Split(src, "\n")

	for _, sec := range v.sections {
		match := findMatchingSection(sec.Name, sections)
		if match == "" {
			conf := 90
			sev := SeverityWarn
			if sec.Required {
				sev = SeverityBlock
				conf = 95
			}
			findings = append(findings, Finding{
				Category:    v.Name(),
				Severity:    sev,
				Title:       "缺少「" + sec.Name + "」模块",
				Description: "大纲中没有找到「" + sec.Name + "」模块。",
				Suggestion:  sec.Tip,
				Confidence:  conf,
			})
			continue
		}
		// Check keyword coverage within the matched section.
		sectionText := extractSectionContent(match, lines)
		missingKW := missingKeywords(sectionText, sec.Keywords)
		if len(missingKW) > 0 {
			sev := SeverityWarn
			if sec.Required {
				sev = SeverityWarn
			}
			findings = append(findings, Finding{
				Category:    v.Name(),
				Severity:    sev,
				Title:       "「" + sec.Name + "」内容不完整",
				Description: "该模块缺少关键要素：" + strings.Join(missingKW, "、"),
				Suggestion:  "补充" + strings.Join(missingKW, "、") + "相关的内容描述",
				Location:    match,
				Confidence:  75,
			})
		}
	}

	// Check bulk / density heuristics.
	if len(lines) < 20 {
		findings = append(findings, Finding{
			Category:    v.Name(),
			Severity:    SeverityWarn,
			Title:       "大纲篇幅过短",
			Description: "当前大纲仅" + formatLineCount(len(lines)) + "，可能缺乏足够的细节支撑正文写作",
			Suggestion:  "扩展每个模块，确保每卷有起承转合和爽点节点",
			Confidence:  70,
		})
	}

	bulletCount := len(v.bulletPattern.FindAllString(src, -1))
	if bulletCount < 10 && len(lines) > 10 {
		findings = append(findings, Finding{
			Category:    v.Name(),
			Severity:    SeverityInfo,
			Title:       "细节点偏少",
			Description: "大纲中使用要点符号的数量偏少（" + formatInt(bulletCount) + "个），可能缺乏具体细节",
			Suggestion:  "在每个模块下使用要点列表细化具体内容",
			Confidence:  60,
		})
	}

	return findings, nil
}

// --- helpers ---

// extractSections finds all 【...】 headings in the text.
func extractSections(src string, pat *regexp.Regexp) []string {
	matches := pat.FindAllString(src, -1)
	for i, m := range matches {
		matches[i] = strings.TrimSpace(m)
	}
	return matches
}

// findMatchingSection checks if any extracted section heading matches name.
func findMatchingSection(name string, sections []string) string {
	for _, s := range sections {
		if strings.Contains(s, name) {
			return s
		}
	}
	return ""
}

// extractSectionContent returns the text between a heading and the next heading
// of the same level, or the end of the file.
func extractSectionContent(heading string, lines []string) string {
	start := -1
	for i, line := range lines {
		if strings.Contains(line, heading) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "【") {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(lines[i])
	}
	return b.String()
}

// missingKeywords returns keywords from the list that are NOT found in text.
func missingKeywords(text string, keywords []string) []string {
	lower := strings.ToLower(text)
	var missing []string
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			missing = append(missing, kw)
		}
	}
	return missing
}

func formatLineCount(n int) string {
	return formatInt(n) + "行"
}

func formatInt(n int) string {
	return fmt.Sprintf("%d", n)
}

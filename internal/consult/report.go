package consult

import (
	"fmt"
	"strings"
)

// FormatReport renders a consultation report as a human-readable Markdown
// string suitable for display in the chat or writing to a file.
func FormatReport(r *Report) string {
	var b strings.Builder

	b.WriteString("═══ 创作咨询报告 ═══\n")
	b.WriteString(fmt.Sprintf("分析对象：%s\n", r.Subject))
	b.WriteString(fmt.Sprintf("健康评分：%d/100\n", r.Score))
	b.WriteString(fmt.Sprintf("总体评估：%s\n", r.Summary))

	if len(r.Findings) == 0 {
		b.WriteString("\n✅ 没有发现问题，当前状态良好！\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("\n共发现 %d 个问题：\n", len(r.Findings)))

	// Group by severity.
	blockers := filterBySeverity(r.Findings, SeverityBlock)
	warnings := filterBySeverity(r.Findings, SeverityWarn)
	infos := filterBySeverity(r.Findings, SeverityInfo)

	if len(blockers) > 0 {
		b.WriteString("\n### 🔴 必须修复（阻塞项）\n")
		for _, f := range blockers {
			writeFinding(&b, f)
		}
	}

	if len(warnings) > 0 {
		b.WriteString("\n### 🟡 建议修复（警告）\n")
		for _, f := range warnings {
			writeFinding(&b, f)
		}
	}

	if len(infos) > 0 {
		b.WriteString("\n### 🔵 参考建议（信息）\n")
		for _, f := range infos {
			writeFinding(&b, f)
		}
	}

	b.WriteString("\n═══ 报告结束 ═══\n")
	return b.String()
}

func writeFinding(b *strings.Builder, f Finding) {
	b.WriteString(fmt.Sprintf("\n**%s**", f.Title))
	if f.Location != "" {
		b.WriteString(fmt.Sprintf("（%s）", f.Location))
	}
	b.WriteString("\n")
	if f.Description != "" {
		b.WriteString(fmt.Sprintf("> %s\n", f.Description))
	}
	if f.Suggestion != "" {
		b.WriteString(fmt.Sprintf("💡 建议：%s\n", f.Suggestion))
	}
	if f.Confidence > 0 {
		bar := confidenceBar(f.Confidence)
		b.WriteString(fmt.Sprintf("可信度：%s %d%%\n", bar, f.Confidence))
	}
}

func filterBySeverity(findings []Finding, sev Severity) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

func confidenceBar(confidence int) string {
	full := confidence / 20
	if full < 1 {
		full = 1
	}
	if full > 5 {
		full = 5
	}
	return strings.Repeat("█", full) + strings.Repeat("░", 5-full)
}

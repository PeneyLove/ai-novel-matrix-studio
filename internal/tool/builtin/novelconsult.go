package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/consult"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(novelConsult{})
}

type novelConsult struct{}

type novelConsultArgs struct {
	Subject string `json:"subject"` // 分析对象描述，如"大纲审核"、"第5章节奏"
	Source  string `json:"source"`  // 待分析的文本内容（从文件读取后传入）
}

func (n novelConsult) Name() string { return "novel_consult" }

func (n novelConsult) Description() string {
	return "内置创作咨询引擎：对大纲/人设/剧情/伏笔/风格进行结构化多源分析，返回量化评分和改进建议。支持多源输入：在 source 中用 '=== source_name ===' 分隔不同文件的内容（如 === outline ===\n...\n=== characters ===\n...）。先 read_file 读取内容，再将内容传入 source 参数。"
}

func (n novelConsult) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"subject": {
				"type": "string",
				"description": "分析对象描述，如'大纲审核'、'第5章节奏'、'人物设定检查'"
			},
			"source": {
				"type": "string",
				"description": "待分析的文本内容。支持多源输入，用 '=== source_name ===' 分隔不同文件的内容。如 === outline ===\n大纲内容\n=== characters ===\n人设内容"
			}
		},
		"required": ["subject", "source"]
	}`)
}

func (n novelConsult) ReadOnly() bool { return true }

func (n novelConsult) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p novelConsultArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("novel_consult: invalid args: %w", err)
	}
	if p.Source == "" {
		return "", fmt.Errorf("novel_consult: source text is empty — read the file first and pass its content")
	}
	if p.Subject == "" {
		p.Subject = "创作内容分析"
	}

	// Detect multi-source format: "=== name ===" section markers.
	if hasMultiSourceMarkers(p.Source) {
		sources := splitMultiSource(p.Source)
		return consult.QuickMultiSourceConsult(p.Subject, sources), nil
	}

	// Single-source fallback.
	report := consult.QuickConsult(p.Subject, p.Source)
	return report, nil
}

// hasMultiSourceMarkers detects whether text contains "=== name ===" markers.
func hasMultiSourceMarkers(text string) bool {
	lines := strings.Split(text, "\n")
	markerCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "===") && strings.HasSuffix(trimmed, "===") && len(trimmed) > 6 {
			markerCount++
		}
	}
	return markerCount >= 1
}

// splitMultiSource splits text by "=== name ===" markers into a name→content map.
func splitMultiSource(text string) map[string]string {
	lines := strings.Split(text, "\n")
	var sections []struct {
		name  string
		start int
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "===") && strings.HasSuffix(trimmed, "===") {
			name := strings.TrimSpace(trimmed[3 : len(trimmed)-3])
			if name != "" {
				sections = append(sections, struct {
					name  string
					start int
				}{name: name, start: i})
			}
		}
	}
	if len(sections) == 0 {
		return map[string]string{"default": text}
	}

	result := make(map[string]string, len(sections))
	for idx, sec := range sections {
		end := len(lines)
		if idx+1 < len(sections) {
			end = sections[idx+1].start
		}
		content := strings.TrimSpace(strings.Join(lines[sec.start+1:end], "\n"))
		result[sec.name] = content
	}
	return result
}

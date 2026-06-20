package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() {
	tool.RegisterBuiltin(batchScan{})
}

type batchScan struct{}

type batchScanArgs struct {
	Chapters []batchScanChapter `json:"chapters"` // 章节列表
}

type batchScanChapter struct {
	ChapterID string `json:"chapter_id"` // 章节标识（如 "第1章"）
	Source    string `json:"source"`     // 章节文本内容（先 read_file 读取后传入）
}

func (b batchScan) Name() string { return "batch_scan" }

func (b batchScan) Description() string {
	return "批量章节扫描：对多章文本并行运行审查清单（字数、段落、标点、句逗比、平坦段、标牌），输出按严重度排序的汇总报告。每章先用 read_file 读取内容，再将所有章节的 {chapter_id, source} 打包传入 chapters 数组。"
}

func (b batchScan) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"chapters": {
				"type": "array",
				"description": "章节列表，每项包含 chapter_id 和 source",
				"items": {
					"type": "object",
					"properties": {
						"chapter_id": {
							"type": "string",
							"description": "章节标识，如'第1章'"
						},
						"source": {
							"type": "string",
							"description": "章节文本内容（从 read_file 读取后传入）"
						}
					},
					"required": ["chapter_id", "source"]
				}
			}
		},
		"required": ["chapters"]
	}`)
}

func (b batchScan) ReadOnly() bool { return true }

func (b batchScan) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p batchScanArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("batch_scan: invalid args: %w", err)
	}
	if len(p.Chapters) == 0 {
		return "", fmt.Errorf("batch_scan: chapters array is empty")
	}

	report := runBatchScan(p.Chapters)
	return report, nil
}

// ---------- batch scan engine ----------

type chapterScanResult struct {
	ChapterID  string
	CJKCount   int
	Over80     int
	FlatLines  int
	SignCount  int
	PeriodQ    int
	Exclam     int
	Ratio      float64
	FailCount  int
	WarnCount  int
}

func runBatchScan(chapters []batchScanChapter) string {
	var results []chapterScanResult

	for _, ch := range chapters {
		text := ch.Source
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		text = strings.TrimPrefix(text, "\uFEFF")

		lines := strings.Split(text, "\n")
		nodes := paragraphNodes(lines)

		r := chapterScanResult{ChapterID: ch.ChapterID}

		// Item 1: 纯汉字
		r.CJKCount = countCJK(text)
		if r.CJKCount < 2300 {
			r.FailCount++
		}

		// Item 2: 超80段
		for _, nd := range nodes {
			if countCJK(nd.text) > 80 {
				r.Over80++
			}
		}
		if r.Over80 > 0 {
			r.FailCount++
		}

		// Item 3: 段间空行
		if countConsecutiveNonBlank(nodes) > 0 {
			r.WarnCount++
		}

		// Item 10: 标点
		r.PeriodQ = len(findQuestionEndingWithPeriod(text))
		r.Exclam = strings.Count(text, "！")
		if r.PeriodQ > 0 || r.Exclam > 5 {
			r.FailCount++
		}

		// Item 12: 标牌
		signs := findTimePlaceSigns(lines)
		r.SignCount = len(signs)
		if r.SignCount > 0 {
			r.FailCount++
		}

		// Item 16: 句逗比
		periodCnt := strings.Count(text, "。")
		commaCnt := strings.Count(text, "，")
		if commaCnt > 0 {
			r.Ratio = float64(periodCnt) / float64(commaCnt)
		} else if periodCnt > 0 {
			r.Ratio = float64(periodCnt) // ∞ effectively, cap for display
		}
		if r.Ratio > 3.0 {
			r.FailCount++
		}

		// Item 17: 平坦段
		r.FlatLines = len(findFlatSegments(nodes))
		if r.FlatLines > 0 {
			r.FailCount++
		}

		results = append(results, r)
	}

	// Sort by severity: failCount desc, then warnCount desc.
	sort.Slice(results, func(i, j int) bool {
		if results[i].FailCount != results[j].FailCount {
			return results[i].FailCount > results[j].FailCount
		}
		return results[i].WarnCount > results[j].WarnCount
	})

	// Build summary report.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("═══ 批量扫描报告：%d 章 ═══\n\n", len(chapters)))

	// Per-chapter table.
	sb.WriteString("| 章节 | 字数 | 超80段 | 平坦段 | 标牌 | 问句。 | 感叹号 | 句逗比 | 违规 |\n")
	sb.WriteString("|------|------|--------|--------|------|--------|--------|--------|------|\n")
	for _, r := range results {
		ratioStr := fmt.Sprintf("%.1f:1", r.Ratio)
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %s | %d |\n",
			r.ChapterID, r.CJKCount, r.Over80, r.FlatLines,
			r.SignCount, r.PeriodQ, r.Exclam, ratioStr, r.FailCount))
	}

	sb.WriteString("\n")

	// Summary statistics.
	totalFail := 0
	totalWarn := 0
	minCJK := 999999
	maxCJK := 0
	maxRatio := 0.0
	minRatio := 999.0
	for _, r := range results {
		totalFail += r.FailCount
		totalWarn += r.WarnCount
		if r.CJKCount < minCJK {
			minCJK = r.CJKCount
		}
		if r.CJKCount > maxCJK {
			maxCJK = r.CJKCount
		}
		if r.Ratio > maxRatio {
			maxRatio = r.Ratio
		}
		if r.Ratio < minRatio && r.Ratio > 0 {
			minRatio = r.Ratio
		}
	}

	sb.WriteString("### 汇总统计\n\n")
	sb.WriteString(fmt.Sprintf("- 总违规数：%d\n", totalFail))
	sb.WriteString(fmt.Sprintf("- 总警告数：%d\n", totalWarn))
	sb.WriteString(fmt.Sprintf("- 字数范围：%d ~ %d\n", minCJK, maxCJK))
	sb.WriteString(fmt.Sprintf("- 句逗比范围：%.1f:1 ~ %.1f:1\n", minRatio, maxRatio))
	if totalFail > 0 {
		sb.WriteString(fmt.Sprintf("- 平均每章违规：%.1f 项\n", float64(totalFail)/float64(len(results))))
	}

	// Top offenders.
	sb.WriteString("\n### 严重度排名（违规数降序）\n\n")
	for i, r := range results {
		if r.FailCount == 0 {
			break
		}
		flag := ""
		if i < 3 {
			flag = " 🔴"
		}
		issues := []string{}
		if r.CJKCount < 2300 {
			issues = append(issues, fmt.Sprintf("字数不足(%d)", r.CJKCount))
		}
		if r.Over80 > 0 {
			issues = append(issues, fmt.Sprintf("超80段×%d", r.Over80))
		}
		if r.FlatLines > 0 {
			issues = append(issues, fmt.Sprintf("平坦段×%d", r.FlatLines))
		}
		if r.SignCount > 0 {
			issues = append(issues, fmt.Sprintf("标牌×%d", r.SignCount))
		}
		if r.PeriodQ > 0 {
			issues = append(issues, fmt.Sprintf("问句用。×%d", r.PeriodQ))
		}
		if r.Ratio > 3.0 {
			issues = append(issues, fmt.Sprintf("句逗比%.1f:1", r.Ratio))
		}
		sb.WriteString(fmt.Sprintf("%d.%s %s：%s\n", i+1, flag, r.ChapterID, strings.Join(issues, " / ")))
	}

	// Chapters with zero violations.
	cleanCount := 0
	for _, r := range results {
		if r.FailCount == 0 {
			cleanCount++
		}
	}
	if cleanCount > 0 {
		sb.WriteString(fmt.Sprintf("\n✅ %d 章零违规：", cleanCount))
		var cleanNames []string
		for _, r := range results {
			if r.FailCount == 0 {
				cleanNames = append(cleanNames, r.ChapterID)
			}
		}
		sb.WriteString(strings.Join(cleanNames, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("\n### 批量修复建议\n\n")
	if totalFail > 0 {
		sb.WriteString("优先修复严重度前 3 章，可按章节分发到 subagent 并行修复。\n")
		sb.WriteString("修复后逐章验证，新发现的通用规则写入 sop-vol1。\n")
	} else {
		sb.WriteString("所有章节自动化检查通过 ✅\n")
	}

	sb.WriteString("\n═══ 报告结束 ═══\n")
	return sb.String()
}

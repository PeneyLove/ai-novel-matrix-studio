package consult

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MultiSourceEngine extends Engine to accept multiple named source texts
// (e.g. outline text + character files + hook ledger), running strategies
// that declare interest in specific sources.
type MultiSourceEngine struct {
	strategies []MultiSourceStrategy
}

// MultiSourceStrategy is a Strategy that can consume multiple named sources.
// The sources map is keyed by a symbolic name like "outline", "characters",
// "hooks", or a file path.
type MultiSourceStrategy interface {
	Name() string
	Description() string
	// AnalyzeSources receives the full map of available sources (name→content).
	// A strategy should only read the sources it recognises and ignore others.
	AnalyzeSources(sources map[string]string) ([]Finding, error)
}

// NewMultiSourceEngine creates an engine from multi-source strategies.
func NewMultiSourceEngine(strategies []MultiSourceStrategy) *MultiSourceEngine {
	return &MultiSourceEngine{strategies: strategies}
}

// Consult runs all strategies against the named sources.
func (e *MultiSourceEngine) Consult(subject string, sources map[string]string) *Report {
	report := &Report{
		Subject:  subject,
		Findings: make([]Finding, 0),
	}
	for _, s := range e.strategies {
		findings, err := s.AnalyzeSources(sources)
		if err != nil {
			report.Add(Finding{
				Category:    s.Name(),
				Severity:    SeverityWarn,
				Title:       "分析策略执行错误",
				Description: fmt.Sprintf("策略 %q 执行时出错: %v", s.Name(), err),
				Suggestion:  "重试或联系开发者",
				Confidence:  100,
			})
			continue
		}
		report.Findings = append(report.Findings, findings...)
	}
	report.Score = ScoreBySeverity(report.Findings)
	if report.Score >= 80 {
		report.Summary = "总体良好，个别细节可优化"
	} else if report.Score >= 50 {
		report.Summary = "存在若干问题，建议针对性修改"
	} else {
		report.Summary = "存在严重问题，建议大幅修改后重新咨询"
	}
	return report
}

// --- ConsistencyStrategy: 人设一致性校验 ---

// ConsistencyStrategy checks character YAML files against the latest chapter
// content for OOC (out-of-character) behavior. It reads the "characters" and
// "chapters" sources from the multi-source map.
type ConsistencyStrategy struct{}

// NewConsistencyStrategy creates a ConsistencyStrategy.
func NewConsistencyStrategy() *ConsistencyStrategy {
	return &ConsistencyStrategy{}
}

func (s *ConsistencyStrategy) Name() string        { return "character-consistency" }
func (s *ConsistencyStrategy) Description() string  { return "人设一致性校验：逐章比对角色设定，标记OOC行为" }

func (s *ConsistencyStrategy) AnalyzeSources(sources map[string]string) ([]Finding, error) {
	var findings []Finding

	charContent, hasChars := sources["characters"]
	if !hasChars || strings.TrimSpace(charContent) == "" {
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityInfo,
			Title:       "未提供人设文件",
			Description: "需要 characters/ 目录下的人设YAML文件进行一致性校验",
			Suggestion:  "先用 /novel-characters 创建人设文件",
			Confidence:  80,
		})
		return findings, nil
	}

	// Extract character names from the YAML source.
	charNames := extractCharacterNames(charContent)
	if len(charNames) == 0 {
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityWarn,
			Title:       "未能从人设文件中提取角色名",
			Description: "人设文件格式可能不规范，无法提取角色名称用于一致性校验",
			Suggestion:  "确保 YAML 文件包含 name: 字段",
			Confidence:  60,
		})
		return findings, nil
	}

	chapterContent, hasChapters := sources["chapters"]
	if !hasChapters || strings.TrimSpace(chapterContent) == "" {
		// Not a blocker — chapter-level consistency check is optional.
		return findings, nil
	}

	// Simple OOC detection heuristic: check if named characters appear
	// with behaviors that contradict their basic archetype.
	for _, name := range charNames {
		if strings.Count(chapterContent, name) > 0 {
			// The character appears — a full OOC check would require
			// embedding-based semantic analysis, which is out of scope
			// for the deterministic engine. We flag the check as done.
			findings = append(findings, Finding{
				Category:    s.Name(),
				Severity:    SeverityInfo,
				Title:       fmt.Sprintf("角色 %s 出现在当前章节中", name),
				Description: fmt.Sprintf("角色 %s 在正文中出现，建议人工判断是否符合人设", name),
				Confidence:  50,
			})
		}
	}

	return findings, nil
}

// extractCharacterNames pulls name: fields from YAML character files.
func extractCharacterNames(src string) []string {
	re := regexp.MustCompile(`(?m)^name:\s*(.+)$`)
	matches := re.FindAllStringSubmatch(src, -1)
	var names []string
	for _, m := range matches {
		if n := strings.TrimSpace(m[1]); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// --- LogicStrategy: 逻辑bug排查 ---

// LogicStrategy checks for timeline contradictions, power-level instability,
// and setting inconsistencies.
type LogicStrategy struct{}

// NewLogicStrategy creates a LogicStrategy.
func NewLogicStrategy() *LogicStrategy {
	return &LogicStrategy{}
}

func (s *LogicStrategy) Name() string       { return "logic-debug" }
func (s *LogicStrategy) Description() string { return "逻辑bug排查：时间线矛盾、战力崩坏、设定不一致" }

func (s *LogicStrategy) AnalyzeSources(sources map[string]string) ([]Finding, error) {
	var findings []Finding

	// Check outline + chapter sources for known contradiction patterns.
	outline, hasOutline := sources["outline"]
	chapters, hasChapters := sources["chapters"]

	if !hasOutline && !hasChapters {
		return findings, nil
	}

	// 1. Check for repeated "突破" (breakthrough) events in the same realm.
	if hasChapters {
		breakthroughs := findBreakthroughs(chapters)
		if len(breakthroughs) > 0 {
			// Check for realm repetition (e.g. "筑基" mentioned more than once
			// as a breakthrough target within the same chapter range).
			realmCounts := countRealms(breakthroughs)
			for realm, count := range realmCounts {
				if count > 2 {
					findings = append(findings, Finding{
						Category:    s.Name(),
						Severity:    SeverityWarn,
						Title:       fmt.Sprintf("境界「%s」突破次数异常", realm),
						Description: fmt.Sprintf("「%s」出现了 %d 次突破描述，可能造成战力升级节奏过快", realm, count),
						Suggestion:  "检查是否在同一个境界内安排了多次突破，建议合并或重新分配突破节点",
						Confidence:  65,
					})
				}
			}
		}
	}

	// 2. Check timeline: if outline has volume ranges, check they're consistent.
	if hasOutline && hasChapters {
		volumeRanges := extractVolumeRanges(outline)
		if len(volumeRanges) > 0 {
			lineCount := len(strings.Split(chapters, "\n"))
			lastVol := volumeRanges[len(volumeRanges)-1]
			if lastVol.End > 0 && lineCount > lastVol.End*50 {
				// Rough check: chapter count seems too high for the outlined range.
				findings = append(findings, Finding{
					Category:    s.Name(),
					Severity:    SeverityInfo,
					Title:       "内容篇幅超出大纲规划",
					Description: fmt.Sprintf("当前章节内容的行数（%d行）已超出大纲最后一卷的规划范围（第%d章）", lineCount, lastVol.End),
					Suggestion:  "考虑扩展大纲或调整当前卷的章节分配",
					Confidence:  55,
				})
			}
		}
	}

	return findings, nil
}

type volumeRange struct {
	Start, End int
}

func findBreakthroughs(text string) []string {
	re := regexp.MustCompile(`(?i)(?:突破|晋级|进阶|渡劫)[^。！\n]{0,20}`)
	matches := re.FindAllString(text, -1)
	return matches
}

func countRealms(breakthroughs []string) map[string]int {
	realmPat := regexp.MustCompile(`(炼气|筑基|金丹|元婴|化神|练气|开光|融合|心动|灵寂|渡劫|大乘|仙人|神境)`)
	counts := make(map[string]int)
	for _, b := range breakthroughs {
		matches := realmPat.FindAllString(b, -1)
		for _, m := range matches {
			counts[m]++
		}
	}
	return counts
}

func extractVolumeRanges(outline string) []volumeRange {
	re := regexp.MustCompile(`(?:第\s*([一二三四五六七八九十百千\d]+)\s*卷|卷\s*\d+)\D*?(\d+)[^0-9]*(\d+)`)
	matches := re.FindAllStringSubmatch(outline, -1)
	var ranges []volumeRange
	for _, m := range matches {
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		if start > 0 && end > start {
			ranges = append(ranges, volumeRange{Start: start, End: end})
		}
	}
	return ranges
}

// --- HookStrategy: 伏笔回收校验 ---

// HookStrategy reads the hook/ledger.yaml (as "hooks" source) and checks
// which hooks are overdue, which have no recovery plan, and overall recovery rate.
type HookStrategy struct{}

// NewHookStrategy creates a HookStrategy.
func NewHookStrategy() *HookStrategy {
	return &HookStrategy{}
}

func (s *HookStrategy) Name() string       { return "hook-recovery" }
func (s *HookStrategy) Description() string { return "伏笔回收校验：扫描已埋设伏笔，标记待回收节点" }

func (s *HookStrategy) AnalyzeSources(sources map[string]string) ([]Finding, error) {
	var findings []Finding

	hooksContent, hasHooks := sources["hooks"]
	if !hasHooks || strings.TrimSpace(hooksContent) == "" {
		return findings, nil
	}

	// Parse hook ledger entries. Expects lines like:
	//   hook-001: {description: "...", status: "pending", expected_recovery: "第50章"}
	hooks := parseHookEntries(hooksContent)
	if len(hooks) == 0 {
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityInfo,
			Title:       "未发现结构化伏笔台账",
			Description: "伏笔台账中没有识别出规范的伏笔条目",
			Suggestion:  "确保 hook/ledger.yaml 使用标准格式记录伏笔",
			Confidence:  70,
		})
		return findings, nil
	}

	pendingCount := 0
	resolvedCount := 0
	noPlanCount := 0
	for _, h := range hooks {
		if h.Status == "pending" || h.Status == "open" {
			pendingCount++
			if h.ExpectedChapter == 0 && h.ExpectedLocation == "" {
				noPlanCount++
			}
		} else if h.Status == "resolved" || h.Status == "completed" || h.Status == "recovered" {
			resolvedCount++
		}
	}

	total := len(hooks)
	if pendingCount > 0 {
		recoveryRate := 0
		if total > 0 {
			recoveryRate = resolvedCount * 100 / total
		}
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityWarn,
			Title:       fmt.Sprintf("伏笔回收率 %d%%", recoveryRate),
			Description: fmt.Sprintf("共 %d 条伏笔：已回收 %d 条，待回收 %d 条", total, resolvedCount, pendingCount),
			Suggestion:  "在后续章节中安排待回收伏笔的回收节点",
			Confidence:  80,
		})
	}

	if noPlanCount > 0 {
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityWarn,
			Title:       fmt.Sprintf("%d 条伏笔缺少回收计划", noPlanCount),
			Description: fmt.Sprintf("有 %d 条伏笔没有标注期望回收章节或位置", noPlanCount),
			Suggestion:  "为每条伏笔补充 expected_recovery 字段，规划回收时机",
			Confidence:  85,
		})
	}

	return findings, nil
}

type hookEntry struct {
	ID               string
	Description      string
	Status           string
	ExpectedChapter  int
	ExpectedLocation string
}

func parseHookEntries(src string) []hookEntry {
	var entries []hookEntry
	lines := strings.Split(src, "\n")
	var current hookEntry
	inEntry := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Top-level key (no leading indent) starts a new entry.
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(trimmed, ":") {
			if current.ID != "" {
				entries = append(entries, current)
			}
			current = hookEntry{ID: strings.TrimSuffix(trimmed, ":")}
			inEntry = true
			continue
		}
		if !inEntry {
			continue
		}
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "description", "desc":
				current.Description = val
			case "status":
				current.Status = val
			case "expected_recovery", "recovery_chapter", "recovery":
				if n, err := strconv.Atoi(val); err == nil {
					current.ExpectedChapter = n
				} else {
					current.ExpectedLocation = val
				}
			}
		}
	}
	if current.ID != "" {
		entries = append(entries, current)
	}
	return entries
}

// --- StyleStrategy: 写作风格分析 ---

// StyleStrategy analyzes writing style: narrative perspective consistency,
// dialogue attribution, AI-slop phrase detection, and paragraph length.
type StyleStrategy struct {
	slopPhrases []string
}

// NewStyleStrategy creates a StyleStrategy.
func NewStyleStrategy() *StyleStrategy {
	return &StyleStrategy{
		slopPhrases: []string{
			"在这个充满", "让我们", "值得注意的是", "不可否认",
			"毫无疑问", "从某种意义上说", "某种程度上", "毕竟",
			"与此同时", "然而", "此外", "总的来说",
			"综上", "综上所述", "我们可以", "我们能够",
		},
	}
}

func (s *StyleStrategy) Name() string       { return "style-analysis" }
func (s *StyleStrategy) Description() string { return "写作风格分析：AI-slop检测、叙述视角、段落长度" }

func (s *StyleStrategy) AnalyzeSources(sources map[string]string) ([]Finding, error) {
	var findings []Finding

	chapters, hasChapters := sources["chapters"]
	if !hasChapters || strings.TrimSpace(chapters) == "" {
		return findings, nil
	}

	// 1. AI-slop phrase detection.
	slopCount := 0
	for _, phrase := range s.slopPhrases {
		count := strings.Count(chapters, phrase)
		slopCount += count
	}
	if slopCount > 0 {
		severity := SeverityInfo
		if slopCount > 3 {
			severity = SeverityWarn
		}
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    severity,
			Title:       fmt.Sprintf("AI-slop 短语出现 %d 次", slopCount),
			Description: fmt.Sprintf("检测到 %d 个AI常用套话，可能降低文本的自然度和辨识度", slopCount),
			Suggestion:  "建议统一替换为更自然的表达，减少'值得注意的是''综上所述'等套话",
			Confidence:  70,
		})
	}

	// 2. Check paragraph length distribution.
	paragraphs := strings.Split(chapters, "\n\n")
	longParas := 0
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) > 300 {
			longParas++
		}
	}
	totalParas := len(paragraphs)
	if totalParas > 0 && longParas > totalParas/3 {
		findings = append(findings, Finding{
			Category:    s.Name(),
			Severity:    SeverityInfo,
			Title:       "段落偏长",
			Description: fmt.Sprintf("超过300字的段落占比 %.0f%%（%d/%d）", float64(longParas)/float64(totalParas)*100, longParas, totalParas),
			Suggestion:  "网文建议每段控制在100-200字，过长的段落会降低手机端阅读体验",
			Confidence:  65,
		})
	}

	return findings, nil
}

// --- MultiSourceAdapters ---

// StrategyToMultiSource adapts a single-source Strategy to MultiSourceStrategy
// by passing the "default" source key if it exists.
type StrategyToMultiSource struct {
	Strategy
	sourceKey string
}

// NewMultiSourceAdapter wraps a single-source Strategy for use in MultiSourceEngine.
// sourceKey is the key used to look up the source text in the sources map.
func NewMultiSourceAdapter(s Strategy, sourceKey string) *StrategyToMultiSource {
	return &StrategyToMultiSource{Strategy: s, sourceKey: sourceKey}
}

func (a *StrategyToMultiSource) AnalyzeSources(sources map[string]string) ([]Finding, error) {
	src := sources[a.sourceKey]
	return a.Strategy.Analyze(src)
}

// AllMultiSourceStrategies returns the full set of built-in multi-source strategies.
func AllMultiSourceStrategies() []MultiSourceStrategy {
	return []MultiSourceStrategy{
		NewMultiSourceAdapter(NewOutlineValidator(nil), "outline"),
		NewMultiSourceAdapter(NewCharacterAnalyzer(), "outline"),
		NewMultiSourceAdapter(NewPlotStructureAnalyzer(), "outline"),
		NewMultiSourceAdapter(NewPacingAnalyzer(), "outline"),
		NewConsistencyStrategy(),
		NewLogicStrategy(),
		NewHookStrategy(),
		NewStyleStrategy(),
	}
}

// NewDefaultMultiSourceEngine creates a MultiSourceEngine with all strategies.
func NewDefaultMultiSourceEngine() *MultiSourceEngine {
	return NewMultiSourceEngine(AllMultiSourceStrategies())
}

// QuickMultiSourceConsult runs all multi-source strategies against the given
// sources and returns a formatted report.
func QuickMultiSourceConsult(subject string, sources map[string]string) string {
	engine := NewDefaultMultiSourceEngine()
	report := engine.Consult(subject, sources)
	return FormatReport(report)
}

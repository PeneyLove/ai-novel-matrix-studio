// Package consult provides a structured consultation engine for novel writing —
// a built-in advisory layer that analyzes outlines, characters, plot structure,
// and pacing, producing actionable improvement suggestions with quantified
// confidence. Unlike the inline skills (which rely on the model's ad-hoc
// judgment), this package uses deterministic checks and structured data models
// so the results are consistent, repeatable, and cache-friendly: the analysis
// logic runs in Go, not in the LLM prompt, so the system prompt stays stable.
package consult

import "fmt"

// --- Core types ---

// Severity rates the importance of a consultation finding.
type Severity int

const (
	SeverityInfo  Severity = iota // cosmetic or optional improvement
	SeverityWarn                   // may cause problems if left unaddressed
	SeverityBlock                  // blocks progress — must be resolved before continuing
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warning"
	case SeverityBlock:
		return "blocker"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// Finding is a single piece of advice produced by a consultation strategy.
type Finding struct {
	Category    string   // e.g. "structure", "character", "plot", "pacing", "hook"
	Severity    Severity // how important this finding is
	Title       string   // short headline, e.g. "主角缺少成长弧线"
	Description string   // detailed explanation of the issue
	Suggestion  string   // concrete actionable fix, e.g. "在第5章加入主角的第一次失败经历"
	Location    string   // where the issue is (file path, chapter number, or "general")
	// Confidence is 0-100 — how sure the analysis is about this finding.
	// Lower confidence means the finding is a heuristic guess; higher means
	// deterministic (e.g. missing required field).
	Confidence int
}

// Report aggregates all findings from a consultation session.
type Report struct {
	SessionID string    // unique id for this consultation
	Subject   string    // what was analyzed, e.g. "outline", "characters", "chapter/第1卷/第5章"
	Findings  []Finding // all findings, sorted by severity then confidence
	Summary   string    // one-line overall assessment
	Score     int       // 0-100 overall health score
}

// Add appends a finding to the report and returns the report for chaining.
func (r *Report) Add(f Finding) *Report {
	r.Findings = append(r.Findings, f)
	return r
}

// ScoreBySeverity computes a weighted score: blockers deduct 30 each, warnings
// 10 each, info 2 each. The base is 100.
func ScoreBySeverity(findings []Finding) int {
	score := 100
	for _, f := range findings {
		switch f.Severity {
		case SeverityBlock:
			score -= 30
		case SeverityWarn:
			score -= 10
		case SeverityInfo:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// --- Strategies ---

// Strategy is a single analysis dimension. Each strategy inspects one aspect
// of the novel and returns its findings. Strategies are designed to be
// composable — running all of them produces a holistic consultation report.
type Strategy interface {
	// Name returns a unique identifier for this strategy, e.g. "outline-completeness".
	Name() string
	// Description returns a human-readable explanation of what this strategy checks.
	Description() string
	// Analyze runs the check against the provided source text and returns findings.
	// src is the raw text of the file or section being analyzed.
	Analyze(src string) ([]Finding, error)
}

// Engine orchestrates multiple consultation strategies against one or more
// source texts, producing a consolidated report.
type Engine struct {
	strategies []Strategy
}

// NewEngine creates a consultation engine with the given strategies.
func NewEngine(strategies []Strategy) *Engine {
	return &Engine{strategies: strategies}
}

// RegisterStrategy adds a strategy to the engine.
func (e *Engine) RegisterStrategy(s Strategy) {
	e.strategies = append(e.strategies, s)
}

// Consult runs all registered strategies against src and consolidates findings
// into a single report. subject is a short description (e.g. "大纲审核").
func (e *Engine) Consult(subject, src string) *Report {
	report := &Report{
		Subject:  subject,
		Findings: make([]Finding, 0),
	}
	for _, s := range e.strategies {
		findings, err := s.Analyze(src)
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

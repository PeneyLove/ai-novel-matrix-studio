package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
)

// Mode controls whether the REPL runs in read-only (Plan) or read-write (Agent) mode.
type Mode int

const (
	AgentMode Mode = iota // full access — generate, write chapters, modify characters
	PlanMode              // read-only — review, plan; writes blocked; auto-plan on complex intents
)

func (m Mode) String() string {
	switch m {
	case AgentMode:
		return "Agent"
	case PlanMode:
		return "Plan✎"
	default:
		return "?"
	}
}

// IsReadOnly returns true if the current mode blocks write operations.
func (m Mode) IsReadOnly() bool { return m == PlanMode }

// AutoPlan controls whether complex intents in Agent mode auto-switch to Plan first.
type AutoPlan int

const (
	AutoPlanOff AutoPlan = iota
	AutoPlanAsk          // prompt user before switching
	AutoPlanOn           // switch automatically
)

// ---- Mode Manager ----

type modeState struct {
	mode     Mode
	autoPlan AutoPlan
}

func newModeState() *modeState {
	return &modeState{mode: AgentMode, autoPlan: AutoPlanAsk}
}

// switchTo changes mode and returns a user-facing description of what happened.
func (ms *modeState) switchTo(m Mode) string {
	if ms.mode == m {
		return fmt.Sprintf("已经处于 %s 模式", m)
	}
	old := ms.mode
	ms.mode = m
	return fmt.Sprintf("已从 %s 切换至 %s 模式", old, m)
}

// toggle cycles between Agent and Plan mode.
func (ms *modeState) toggle() string {
	var next Mode
	if ms.mode == AgentMode {
		next = PlanMode
	} else {
		next = AgentMode
	}
	ms.mode = next
	return fmt.Sprintf("已切换至 %s 模式", next)
}

// WriteOperation names the write op being attempted, for user-facing messages.
type WriteOperation string

const (
	WriteChapter  WriteOperation = "续写章节"
	WriteChar     WriteOperation = "创建/修改角色"
	WriteKillChar WriteOperation = "下线角色"
	WriteWorld    WriteOperation = "修改世界观"
	WriteOutline  WriteOperation = "修改大纲"
)

// checkWritePermission returns nil if the operation is permitted, or an error
// that the caller should print and return.
func (s *Session) checkWritePermission(op WriteOperation) error {
	if !s.modeMgr.mode.IsReadOnly() {
		return nil
	}
	fmt.Printf("\n  ⚠ Plan模式不允许执行「%s」操作。\n", op)
	fmt.Print("  切换到 Agent 模式？[Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(strings.ToLower(resp))
	if resp == "y" || resp == "yes" || resp == "" {
		s.modeMgr.switchTo(AgentMode)
		s.printStatusBar()
		fmt.Println("  ✓ 已切换到 Agent 模式，正在执行...")
		return nil
	}
	return fmt.Errorf("操作已取消（仍在 Plan 模式）")
}

// ---- Status Bar ----

func (s *Session) statusBar() string {
	projectName := s.project
	if projectName == "" {
		projectName = "（无项目）"
	}

	chCount := 0
	if s.project != "" {
		chDir := project.Dir(s.root, s.project) + "/output"
		if entries, err := os.ReadDir(chDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
					chCount++
				}
			}
		}
	}

	modelName := ""
	for _, p := range []string{"deepseek", "mimo", "minimax"} {
		if _, err := s.router.GetClient(p); err == nil {
			modelName = p
			break
		}
	}
	if modelName == "" {
		modelName = "（无模型）"
	}

	modeStr := s.modeMgr.mode.String()
	autoStr := ""
	if s.modeMgr.autoPlan == AutoPlanOn {
		autoStr = " [AutoPlan]"
	} else if s.modeMgr.autoPlan == AutoPlanAsk {
		autoStr = " [AutoPlan:Ask]"
	}

	return fmt.Sprintf("📁 %s  ✍ %d章  🤖 %s  [%s]%s",
		projectName, chCount, modelName, modeStr, autoStr)
}

func (s *Session) printStatusBar() {
	fmt.Printf("\r\033[K%s", s.statusBar())
}

// ---- Plan Mode: Auto-planning ----

// generatePlan calls AI to create a short plan for the user's intent.
func (s *Session) generatePlan(intent string) (string, error) {
	// Try any available model
	client, err := s.router.GetClient("deepseek")
	if err != nil {
		client, err = s.router.GetClient("mimo")
		if err != nil {
			client, err = s.router.GetClient("minimax")
			if err != nil {
				return intent, nil // no model → pass through
			}
		}
	}

	systemPrompt := `你是小说创作策划助手。用户在 Agent 模式下输入了创作意图。
你的任务是生成一个简洁的执行计划（≤5步），不需要执行。

格式：
[Plan]
1. 涉及模块：大纲/角色/世界观/章节（只列会被修改的）
2. 具体步骤（最多5步，每步一句话）
3. 预期效果（一句话）

完成后标注：

---
请用户确认后执行。输入 /mode agent 切换回 Agent 模式继续创作。`

	ctx := context.Background()
	plan, err := client.Generate(ctx, systemPrompt, "用户意图："+intent+"\n请生成执行计划。")
	if err != nil {
		return intent, nil
	}
	return plan, nil
}

// maybeAutoPlan checks if the user intent should trigger auto-planning.
func (s *Session) maybeAutoPlan(intent string) string {
	if s.modeMgr.mode != AgentMode {
		return intent
	}
	if s.modeMgr.autoPlan == AutoPlanOff {
		return intent
	}

	// Detect complex intents: has keywords indicating multi-step changes
	complexKeywords := []string{
		"改", "修改", "调整", "重构", "重新设计",
		"删", "删除", "移除",
		"大纲", "世界观", "境界", "势力",
		"重新写", "重写", "翻新",
	}
	isComplex := false
	for _, kw := range complexKeywords {
		if strings.Contains(intent, kw) {
			isComplex = true
			break
		}
	}
	if !isComplex {
		return intent
	}

	if s.modeMgr.autoPlan == AutoPlanAsk {
		fmt.Printf("\n  💡 检测到复杂意图，建议先进入 Plan 模式出方案。切换到 Plan 模式？[Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" && resp != "" {
			return intent
		}
	}

	// Switch to Plan and generate
	s.modeMgr.switchTo(PlanMode)
	s.printStatusBar()
	fmt.Println("\n  📋 正在生成执行计划...")

	plan, err := s.generatePlan(intent)
	if err != nil {
		fmt.Printf("  ⚠ 生成计划失败: %v\n", err)
		return intent
	}

	fmt.Println("\n  ╔══════════════════════════════════════╗")
	fmt.Println("  ║  [Plan] 执行计划                      ║")
	fmt.Println("  ╚══════════════════════════════════════╝")
	fmt.Println(plan)
	fmt.Println("  ────────────────────────────────────────")
	fmt.Println("  确认无误后输入 /mode agent 执行，或直接输入修改反馈。")

	return intent // return original intent; Plan has been displayed
}

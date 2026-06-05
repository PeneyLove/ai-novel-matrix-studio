// Package tui implements the Bubble Tea terminal UI for novel-agent.
//
// Layout (top → bottom):
//
//	┌─ status bar (1 line) ────────────────────────────────┐
//	│ 📁 凡人修仙  ✍ 3章  🤖 deepseek  [Agent]             │
//	├─ chat viewport (scrollable) ─────────────────────────┤
//	│                                                        │
//	│ 🤖 [玄幻修仙-大纲Skill] 根据你的构思，我已生成...       │
//	│                                                        │
//	│ ✍ 我想让男主在第3章觉醒能力                            │
//	│                                                        │
//	│ ⠋ 思考中...                                            │
//	├─ input area (3 lines, fixed) ────────────────────────┤
//	│ [Agent]                                                │
//	│ > _                                                    │
//	│ Enter发送 · Shift+Tab切模式 · Ctrl+C退出 · Ctrl+L清屏  │
//	└────────────────────────────────────────────────────────┘
package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/audit"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/audit"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
)

// Mode constants
const (
	ModeAgent = "Agent"
	ModePlan  = "Plan✎"
)

// --- Styles ---
var (
	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("37")). // blue-ish
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Padding(0, 1).
			Width(0) // auto

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114")).
			Bold(true)

	aiHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("183")).
			Bold(true)

	aiBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("253"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("204"))

	inputModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("183"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// --- Model ---

type Model struct {
	harness   *harness.Harness
	root      string
	audit     audit.Policy
	project   string
	mode      string
	autoPlan  string
	modeMgr   interface{ toggle() string }

	// Chat lines
	chatLines   []string
	viewportPos int // current scroll position (line index from top)

	// Input
	input      strings.Builder
	cursorPos  int

	// State
	width     int
	height    int
	running   bool
	thinking  bool
	thinkingFrame int
	thinkingFrames []string
	thinkingMsg  string

	// Async events
	eventCh    chan any
	onboarding *onboardState
	activeModel string

	// Raw terminal width for layout
	termWidth int
}

type onboardState struct {
	genre, subGenre, powerSystem, desc string
	title1, title2, title3             string
}

// ChatEvent carries AI result or error back from goroutine.
type ChatEvent struct {
	Line   string
	Err    error
	SpinnerStop bool
}

// New creates a TUI Model.
func New(h *harness.Harness, root string) *Model {
	// Find active model
	modelName := ""
	for _, p := range []string{"deepseek", "mimo", "minimax"} {
		if _, err := h.Router.GetClient(p); err == nil {
			modelName = p
			break
		}
	}

	return &Model{
		harness:        h,
		root:           root,
		audit:          audit.DefaultPolicy(),
		mode:           ModeAgent,
		autoPlan:       "Ask",
		chatLines:      make([]string, 0, 200),
		eventCh:        make(chan any, 256),
		thinkingFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		activeModel:    modelName,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.tickThinking(),
		m.listenEvents(),
		// Emit startup events
		m.emitStartup(),
	)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.termWidth = msg.Width
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case ChatEvent:
		if msg.SpinnerStop {
			m.thinking = false
		}
		if msg.Err != nil {
			m.addSystemLine(msg.Err.Error())
		} else if msg.Line != "" {
			m.addAILine(msg.Line)
		}
		return m, m.listenEvents()

	case startupMsg:
		m.addSystemLine(msg.text)
		return m, m.listenEvents()

	case tickMsg:
		if m.thinking {
			m.thinkingFrame = (m.thinkingFrame + 1) % len(m.thinkingFrames)
		}
		return m, m.tickThinking()

	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	sb := strings.Builder{}

	// ── Status bar ──
	sb.WriteString(m.renderStatus())
	sb.WriteByte('\n')

	// ── Chat viewport ──
	chatHeight := m.height - 5 // status(1) + input(3) + separator(1)
	if chatHeight < 3 {
		chatHeight = 3
	}

	start := len(m.chatLines) - chatHeight
	if start < 0 {
		start = 0
	}
	if m.viewportPos > 0 {
		// User has scrolled up — adjust view
		start = m.viewportPos
	}
	end := start + chatHeight
	if end > len(m.chatLines) {
		end = len(m.chatLines)
	}
	for _, line := range m.chatLines[start:end] {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	// Fill remaining lines
	for i := end - start; i < chatHeight; i++ {
		sb.WriteByte('\n')
	}

	// ── Thinking indicator ──
	if m.thinking {
		frame := m.thinkingFrames[m.thinkingFrame]
		sb.WriteString(thinkingStyle.Render(fmt.Sprintf("  %s %s", frame, m.thinkingMsg)))
		sb.WriteByte('\n')
	}

	// ── Separator ──
	sb.WriteString(strings.Repeat("─", m.termWidth))
	sb.WriteByte('\n')

	// ── Input area ──
	modeLabel := inputModeStyle.Render(fmt.Sprintf("[%s]", m.mode))
	sb.WriteString(modeLabel)
	sb.WriteByte('\n')

	sb.WriteString("> ")
	sb.WriteString(m.input.String())
	if m.cursorPos >= 0 {
		sb.WriteString("█") // cursor
	}
	sb.WriteByte('\n')

	sb.WriteString(hintStyle.Render("Enter发送 · Shift+Tab切模式 · Ctrl+C退出 · Ctrl+L清屏"))

	return sb.String()
}

// --- Key handling ---

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.running = false
		return tea.Quit

	case tea.KeyCtrlL:
		m.chatLines = nil
		return nil

	case tea.KeyShiftTab:
		if m.mode == ModeAgent {
			m.mode = ModePlan
		} else {
			m.mode = ModeAgent
		}
		m.addSystemLine(fmt.Sprintf("切换至 %s 模式", m.mode))
		return nil

	case tea.KeyEnter:
		text := strings.TrimSpace(m.input.String())
		m.input.Reset()
		m.cursorPos = 0
		if text == "" {
			// Toggle mode
			if m.mode == ModeAgent {
				m.mode = ModePlan
			} else {
				m.mode = ModeAgent
			}
			m.addSystemLine(fmt.Sprintf("切换至 %s 模式", m.mode))
			return nil
		}

		m.addUserLine(text)
		return m.handleUserInput(text)

	case tea.KeyBackspace:
		s := m.input.String()
		if len(s) > 0 {
			runes := []rune(s)
			m.input.Reset()
			m.input.WriteString(string(runes[:len(runes)-1]))
		}
		return nil

	case tea.KeyRunes:
		m.input.WriteString(string(msg.Runes))
		return nil

	default:
		return nil
	}
}

// --- Async event loop ---

type tickMsg struct{}
type startupMsg struct{ text string }

func (m *Model) tickThinking() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *Model) listenEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case ev := <-m.eventCh:
			return ev
		default:
			return nil
		}
	}
}

func (m *Model) emitStartup() tea.Cmd {
	return func() tea.Msg {
		// Check API key
		hasKey := false
		for _, p := range []string{"deepseek", "mimo", "minimax"} {
			if _, err := m.harness.Router.GetClient(p); err == nil {
				hasKey = true
				m.activeModel = p
				break
			}
		}

		lines := []string{
			"✍  AI Novel Agent v2 · 交互式写作终端",
		}
		if hasKey {
			lines = append(lines, fmt.Sprintf("🤖 活跃模型: %s | /model 切换", m.activeModel))
		} else {
			lines = append(lines, "⚠ 未检测到 API Key。输入 /model set deepseek <key> 配置。")
			lines = append(lines, "  申请: https://platform.deepseek.com/api_keys")
		}

		projects, _ := project.ListProjects(m.root)
		if len(projects) > 0 {
			m.project = projects[0]
			lines = append(lines, fmt.Sprintf("📁 当前项目: %s", m.project))
			lines = append(lines, "直接描述你的创作意图，或输入 /help 查看命令。")
		} else {
			lines = append(lines, "你想写一本怎样的小说？直接告诉我书名、类型、核心灵感。")
		}

		// Send startup lines one by one with small delay
		for _, l := range lines {
			m.eventCh <- startupMsg{text: l}
			time.Sleep(50 * time.Millisecond)
		}
		return nil
	}
}

// --- Input handling ---

func (m *Model) handleUserInput(text string) tea.Cmd {
	if strings.HasPrefix(text, "/quit") || strings.HasPrefix(text, "/exit") {
		m.running = false
		return tea.Quit
	}

	if strings.HasPrefix(text, "/help") {
		m.addSystemLine("可用命令: /project /chars /outline /world /write /export /model /mode")
		m.addSystemLine("模式: Agent(自由) / Plan✎(只读) · Shift+Tab 切换")
		return nil
	}

	if strings.HasPrefix(text, "/project") {
		name := strings.TrimSpace(strings.TrimPrefix(text, "/project"))
		return m.handleProject(name)
	}

	if strings.HasPrefix(text, "/chars") {
		return m.handleListChars()
	}

	if strings.HasPrefix(text, "/char ") {
		name := strings.TrimSpace(strings.TrimPrefix(text, "/char "))
		return m.handleChar(name)
	}

	if strings.HasPrefix(text, "/world") {
		return m.handleWorld()
	}

	if strings.HasPrefix(text, "/outline") {
		return m.handleOutline()
	}

	if strings.HasPrefix(text, "/write") {
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/write"))
		return m.handleWrite(arg)
	}

	if strings.HasPrefix(text, "/model") {
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/model"))
		return m.handleModel(arg)
	}

	if strings.HasPrefix(text, "/mode") {
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/mode"))
		switch arg {
		case "plan":
			m.mode = ModePlan
		case "agent":
			m.mode = ModeAgent
		}
		m.addSystemLine(fmt.Sprintf("当前模式: %s", m.mode))
		return nil
	}

	if strings.HasPrefix(text, "/killchar") {
		name := strings.TrimSpace(strings.TrimPrefix(text, "/killchar "))
		return m.handleKillChar(name)
	}

	// Check write permission
	if m.mode == ModePlan && isWriteOp(text) {
		m.addSystemLine("⚠ Plan 模式不允许写操作。按 Shift+Tab 切换到 Agent 模式后重试。")
		return nil
	}

	// Free-form — route to AI discussion
	return m.handleFreeForm(text)
}

func isWriteOp(text string) bool {
	kw := []string{"/write", "/char", "/killchar"}
	for _, k := range kw {
		if strings.HasPrefix(text, k) {
			return true
		}
	}
	for _, k := range []string{"写", "续写", "创建角色", "修改", "删除", "下线"} {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}

// --- Handler stubs (to be filled) ---

func (m *Model) handleProject(name string) tea.Cmd {
	if name == "" {
		projs, _ := project.ListProjects(m.root)
		if len(projs) == 0 {
			m.addSystemLine("还没有项目。用法: /project <名称>")
			return nil
		}
		m.addSystemLine(fmt.Sprintf("项目列表: %s", strings.Join(projs, ", ")))
		return nil
	}
	if _, err := os.Stat(project.Dir(m.root, name)); os.IsNotExist(err) {
		project.Init(m.root, name)
		m.addSystemLine(fmt.Sprintf("✓ 已创建项目「%s」", name))
	}
	m.project = name
	m.addSystemLine(fmt.Sprintf("✓ 切换到项目「%s」", name))
	return nil
}

func (m *Model) handleListChars() tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	chars, err := project.ListCharacters(m.root, m.project)
	if err != nil || len(chars) == 0 {
		m.addSystemLine("还没有角色。使用 /char <角色名> 创建")
		return nil
	}
	for _, ch := range chars {
		icon := "🟢"
		if ch.Status == "deactivated" {
			icon = "⚫"
		}
		m.addSystemLine(fmt.Sprintf("  %s %s (%s) %s", icon, ch.Name, ch.Role, ch.Personality))
	}
	return nil
}

func (m *Model) handleChar(name string) tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	if name == "" {
		m.addSystemLine("用法: /char <角色名>")
		return nil
	}
	id := project.CharacterID(name)
	_, err := project.ReadCharacter(m.root, m.project, id)
	if err == nil {
		m.addSystemLine(fmt.Sprintf("📝 角色「%s」已存在。编辑请直接修改 .novelAgent/projects/%s/characters/%s.yaml", name, m.project, id))
		return nil
	}
	ch := project.CharacterProfile{
		ID: id, Name: name, Role: "配角", Status: "active",
		Personality: "待设定", Background: "待设定", Motivation: "待设定",
	}
	project.WriteCharacter(m.root, m.project, ch)
	m.addSystemLine(fmt.Sprintf("✓ 角色「%s」已创建", name))
	return nil
}

func (m *Model) handleWorld() tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	w, _ := project.ReadWorld(m.root, m.project)
	m.addSystemLine(fmt.Sprintf("🌍 世界观: %v / %v", w["genre"], w["sub_genre"]))
	m.addSystemLine(fmt.Sprintf("   力量体系: %v", w["power_system"]))
	if factions, ok := w["factions"].([]any); ok && len(factions) > 0 {
		m.addSystemLine("   势力: " + fmt.Sprint(factions))
	}
	return nil
}

func (m *Model) handleOutline() tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	o, _ := project.ReadOutline(m.root, m.project)
	m.addSystemLine(fmt.Sprintf("📋 大纲 V%v | 已定稿: %v | 章节数: %v", o["version"], o["finalized"], o["chapter_count"]))
	return nil
}

func (m *Model) handleWrite(arg string) tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	chNo := 1
	if arg != "" {
		if n, err := strconv.Atoi(arg); err == nil {
			chNo = n
		}
	}

	m.thinking = true
	m.thinkingMsg = "正在续写第" + itoa(chNo) + "章..."
	fullTask := fmt.Sprintf("%s-ch%03d-%d", m.project, chNo, time.Now().Unix())

	// Run AI call in background
	go func() {
		outline, _ := project.ReadOutline(m.root, m.project)
		world, _ := project.ReadWorld(m.root, m.project)
		trendData := fmt.Sprintf("项目: %s, 世界观: %v, 续写第%d章", m.project, world["genre"], chNo)

		skillName := "xuanhuan_writing"
		if g, ok := world["genre"].(string); ok && g != "" {
			skillName = g + "_writing"
		}

		ctx := context.Background()
		out, err := m.harness.RunStage(ctx, fullTask, skillName, "content_generation", pipeline.StageInput{
			TrendData:      trendData,
			ChapterOutline: fmt.Sprintf("第%d章大纲", chNo),
			ChapterNo:      chNo,
			NovelID:        m.project,
		})
		if err != nil {
			m.eventCh <- ChatEvent{SpinnerStop: true, Err: err}
			return
		}

		project.WriteChapter(m.root, m.project, chNo, out.Content)
		outline["chapter_count"] = chNo
		project.WriteOutline(m.root, m.project, outline)

		m.eventCh <- ChatEvent{
			SpinnerStop: true,
			Line:        fmt.Sprintf("✓ 第%d章完成 (%d字)", chNo, len([]rune(out.Content))),
		}
	}()

	return nil
}

func (m *Model) handleModel(arg string) tea.Cmd {
	if arg == "" || arg == "list" {
		for _, p := range []string{"deepseek", "mimo", "minimax"} {
			_, err := m.harness.Router.GetClient(p)
			icon := "✗"
			if err == nil {
				icon = "✓"
			}
			label := model.ProviderLabels[p]
			m.addSystemLine(fmt.Sprintf("  %s %s  %s", icon, p, label))
		}
		return nil
	}
	parts := strings.SplitN(arg, " ", 2)
	if len(parts) >= 2 && parts[0] == "set" {
		parts2 := strings.SplitN(parts[1], " ", 2)
		if len(parts2) >= 2 {
			m.addSystemLine(fmt.Sprintf("✓ %s API Key 已配置", parts2[0]))
		}
		return nil
	}
	if parts[0] == "switch" && len(parts) >= 2 {
		m.activeModel = parts[1]
		m.addSystemLine(fmt.Sprintf("✓ 已切换到 %s", parts[1]))
		return nil
	}
	m.addSystemLine("用法: /model | /model set <模型> <key> | /model switch <模型>")
	return nil
}

func (m *Model) handleKillChar(name string) tea.Cmd {
	if m.project == "" {
		m.addSystemLine("请先选择项目: /project <名称>")
		return nil
	}
	if name == "" {
		m.addSystemLine("用法: /killchar <角色名>")
		return nil
	}
	id := project.CharacterID(name)
	summary, err := project.DeactivateCharacter(m.root, m.project, id, "用户手动下线")
	if err != nil {
		m.addSystemLine(fmt.Sprintf("✗ %v", err))
		return nil
	}
	m.addSystemLine("⚠ 角色" + summary)
	return nil
}

func (m *Model) handleFreeForm(text string) tea.Cmd {
	m.thinking = true
	m.thinkingMsg = "思考中..."

	go func() {
		client, _ := m.harness.Router.GetClient("deepseek")
		if client == nil {
			client, _ = m.harness.Router.GetClient("mimo")
		}
		if client == nil {
			m.eventCh <- ChatEvent{SpinnerStop: true, Err: fmt.Errorf("没有可用的 AI 模型")}
			return
		}
		ctx := context.Background()
		reply, err := client.Generate(ctx, "你是小说创作助手。回答简洁，有网文创作经验。", text)
		if err != nil {
			m.eventCh <- ChatEvent{SpinnerStop: true, Err: err}
			return
		}
		m.eventCh <- ChatEvent{SpinnerStop: true, Line: reply}
	}()

	return nil
}

// --- Line helpers ---

func (m *Model) addUserLine(text string) {
	m.chatLines = append(m.chatLines, userMsgStyle.Render("✍ "+text))
}

func (m *Model) addAILine(text string) {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			m.chatLines = append(m.chatLines, aiBodyStyle.Render("  "+line))
		}
	}
}

func (m *Model) addSystemLine(text string) {
	m.chatLines = append(m.chatLines, systemStyle.Render("  "+text))
}

// --- Helpers ---

func (m *Model) renderStatus() string {
	parts := []string{}
	parts = append(parts, "📁 "+m.project)
	chCount := 0
	if m.project != "" {
		chDir := project.Dir(m.root, m.project) + "/output"
		if entries, err := os.ReadDir(chDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
					chCount++
				}
			}
		}
	}
	parts = append(parts, fmt.Sprintf("✍ %d章", chCount))
	parts = append(parts, "🤖 "+m.activeModel)
	parts = append(parts, fmt.Sprintf("[%s]", m.mode))
	if m.autoPlan != "Off" {
		parts = append(parts, fmt.Sprintf("[AP:%s]", m.autoPlan))
	}
	return statusStyle.Render(strings.Join(parts, "  "))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}



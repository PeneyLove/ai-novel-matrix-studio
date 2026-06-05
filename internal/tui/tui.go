// Package tui implements a zero-dependency terminal UI using raw ANSI/VT100
// escape sequences. Same 3-section layout (status + chat + input) as Bubble Tea
// but without any external packages — single binary, no network flakiness.
//
// Layout:
//
//	┌─ status bar ───────────────────────────────────────┐
//	│ 📁 凡人修仙  ✍ 3章  🤖 deepseek  [Agent]           │
//	├─ chat (scrollable) ────────────────────────────────┤
//	│ ✍ user message                                     │
//	│ 🤖 AI response                                     │
//	│ ⠋ 思考中...                                        │
//	├─ input ────────────────────────────────────────────┤
//	│ > typing...█                                       │
//	│ Enter发送 · Shift+Tab切模式 · Ctrl+C退出            │
//	└─────────────────────────────────────────────────────┘
package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/audit"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
)

// ANSI escapes
const (
	esc       = "\033["
	cursorOff = esc + "?25l"
	cursorOn  = esc + "?25h"
	clearAll  = esc + "2J"
	clearLine = esc + "2K"
	posHome   = esc + "H"
	bgBlue    = esc + "44m"   // status bar background
	fgWhite   = esc + "37m"   // status bar text
	fgPurple  = esc + "35m"   // AI header
	fgGreen   = esc + "32m"   // user text
	fgGray    = esc + "90m"   // system/hint text
	fgRed     = esc + "31m"   // error
	fgYellow  = esc + "33m"   // warning
	reset     = esc + "0m"
	bold      = esc + "1m"
	dim       = esc + "2m"
)

// Mode constants
const (
	ModeAgent = "Agent"
	ModePlan  = "Plan✎"
)

// Spinner frames
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// --- Model ---

type Model struct {
	mu sync.Mutex

	harness     *harness.Harness
	root        string
	project     string
	mode        string
	autoPlan    string
	activeModel string

	// Chat buffer (ring)
	chatLines []chatLine
	chatCap   int

	// Input
	inputBuf []rune
	cursor   int

	// Display
	termW int
	termH int

	// Thinking state
	thinking      bool
	thinkingFrame int
	thinkingMsg   string

	// Lifecycle
	running bool
	reader  *bufio.Reader

	// Async
	eventCh   chan any
	pendingFn func() // next render callback
}

type chatLine struct {
	Type string // "system", "user", "ai", "aiHeader", "error", "warning"
	Text string
}

type ChatEvent struct {
	Line        string
	Err         error
	StopSpinner bool
}

// --- Constructor ---

func New(h *harness.Harness, root string) *Model {
	modelName := ""
	for _, p := range []string{"deepseek", "mimo", "minimax"} {
		if _, err := h.Router.GetClient(p); err == nil {
			modelName = p
			break
		}
	}

	return &Model{
		harness:     h,
		root:        root,
		mode:        ModeAgent,
		autoPlan:    "Ask",
		activeModel: modelName,
		chatLines:   make([]chatLine, 0, 500),
		chatCap:     1000,
		inputBuf:    make([]rune, 0, 4096),
		eventCh:     make(chan any, 256),
		running:     true,
	}
}

// --- Render ---

func (m *Model) render(out *os.File) {
	// Re-detect terminal size every render (no-op on unchanged)
	if w, h, err := getTermSize(); err == nil && w > 0 {
		m.termW = w
		m.termH = h
	}

	var sb strings.Builder

	// Clear screen + cursor off + home
	sb.WriteString(clearAll)
	sb.WriteString(cursorOff)
	sb.WriteString(posHome)

	// ── Status bar ──
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
	statusLeft := fmt.Sprintf("📁 %s  ✍ %d章  🤖 %s  [%s]",
		m.project, chCount, m.activeModel, m.mode)
	if statusLeft == "📁   ✍ 0章  🤖   [Agent]" {
		statusLeft = "📁 （新项目） ✍ 0章  🤖 " + m.activeModel + "  [Agent]"
	}
	status := bgBlue + fgWhite + bold + " " + statusLeft
	pad := m.termW - len([]rune(statusLeft)) - 3
	if pad < 0 {
		pad = 0
	}
	status += strings.Repeat(" ", pad) + reset
	sb.WriteString(status)
	sb.WriteByte('\n')

	// ── Chat area ──
	chatHeight := m.termH - 4 // status(1) + input(3) + thinking(1 optional)
	if chatHeight < 3 {
		chatHeight = 3
	}

	// Calculate visible range
	start := len(m.chatLines) - chatHeight
	if start < 0 {
		start = 0
	}

	for i := start; i < len(m.chatLines); i++ {
		cl := m.chatLines[i]
		switch cl.Type {
		case "system":
			sb.WriteString(fgGray + "  " + cl.Text + reset)
		case "user":
			sb.WriteString(fgGreen + bold + "✍ " + cl.Text + reset)
		case "ai":
			for _, line := range strings.Split(cl.Text, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					sb.WriteString("  " + line)
				}
			}
		case "aiHeader":
			sb.WriteString(fgPurple + bold + "🤖 [" + cl.Text + "]" + reset)
		case "error":
			sb.WriteString(fgRed + "✗ " + cl.Text + reset)
		case "warning":
			sb.WriteString(fgYellow + "⚠ " + cl.Text + reset)
		}
		sb.WriteByte('\n')
	}

	// Fill remaining chat lines
	for i := len(m.chatLines) - start; i < chatHeight; i++ {
		sb.WriteByte('\n')
	}

	// ── Thinking indicator ──
	if m.thinking {
		frame := spinnerFrames[m.thinkingFrame]
		sb.WriteString(dim + fgGray + "  " + frame + " " + m.thinkingMsg + reset)
	}
	sb.WriteByte('\n')

	// ── Separator ──
	sep := strings.Repeat("─", m.termW)
	sb.WriteString(fgGray + sep + reset)
	sb.WriteByte('\n')

	// ── Input ──
	sb.WriteString(fgPurple + "[" + m.mode + "]" + reset)
	sb.WriteByte('\n')
	sb.WriteString("> ")
	sb.WriteString(string(m.inputBuf))
	sb.WriteString("█") // cursor
	sb.WriteByte('\n')

	hint := "Enter发送 · Shift+Tab切模式 · Ctrl+C退出 · Ctrl+L清屏  "
	hint = fgGray + dim + hint + strings.Repeat(" ", m.termW-len([]rune(hint))) + reset
	sb.WriteString(hint)

	out.WriteString(sb.String())
}

// --- Run: main loop ---

func Run(h *harness.Harness, root string) error {
	m := New(h, root)

	// Get terminal size
	if w, h, err := getTermSize(); err == nil {
		m.termW = w
		m.termH = h
	} else {
		m.termW = 80
		m.termH = 24
	}

	// Switch to raw mode
	raw, err := enableRawMode()
	if err != nil {
		return fmt.Errorf("terminal raw mode: %w", err)
	}
	defer disableRawMode(raw)

	// Startup
	m.addSystem("✍  AI Novel Agent v2 · 交互式写作终端")
	m.addSystem("直接描述你的小说构思开始创作，或输入 /help 查看命令")
	m.addSystem("")

	projects, _ := project.ListProjects(root)
	if len(projects) > 0 {
		m.project = projects[0]
		m.addSystem("📁 当前项目: " + m.project)
	} else {
		m.addSystem("你想写一本怎样的小说？")
	}

	m.render(os.Stdout)

	// Input goroutine
	inputCh := make(chan string, 32)
	go m.readInput(inputCh)

	// Spinner ticker
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Main event loop
	for m.running {
		select {
		case <-ticker.C:
			if m.thinking {
				m.mu.Lock()
				m.thinkingFrame = (m.thinkingFrame + 1) % len(spinnerFrames)
				m.mu.Unlock()
				m.render(os.Stdout)
			}

		case input := <-inputCh:
			if input == "\x00" {
				// Keystroke refresh (backspace/char) — inline echo only
				m.echoInput()
				continue
			}
			if input == "\t" {
				// Tab received but not Shift+Tab — ignore
				continue
			}
			if input == "\x01" {
				// Mode changed via Shift+Tab — full render
				m.render(os.Stdout)
				continue
			}
			m.handleInput(input)
			m.render(os.Stdout)

		case ev := <-m.eventCh:
			switch e := ev.(type) {
			case ChatEvent:
				m.mu.Lock()
				if e.StopSpinner {
					m.thinking = false
				}
				if e.Err != nil {
					m.addError(e.Err.Error())
				} else if e.Line != "" {
					m.addAI(e.Line)
				}
				m.mu.Unlock()
				m.render(os.Stdout)
			}
		}
	}

	// Restore cursor + clear screen
	fmt.Fprint(os.Stdout, cursorOn+clearAll+posHome)
	return nil
}

func (m *Model) readInput(ch chan<- string) {
	reader := bufio.NewReader(os.Stdin)
	for m.running {
		m.mu.Lock()
		m.render(os.Stdout)
		m.mu.Unlock()

		// Read byte by byte for key control
		b, err := reader.ReadByte()
		if err != nil {
			ch <- "/quit"
			return
		}

		switch b {
		case 3: // Ctrl+C
			ch <- "/quit"
			return
		case 12: // Ctrl+L
			m.mu.Lock()
			m.chatLines = nil
			m.mu.Unlock()
			ch <- "\x01" // full render needed
			continue
		case 127: // Backspace
			m.mu.Lock()
			if len(m.inputBuf) > 0 {
				m.inputBuf = m.inputBuf[:len(m.inputBuf)-1]
			}
			m.mu.Unlock()
			ch <- "\x00"
			continue
		case 13: // Enter
			text := string(m.inputBuf)
			m.mu.Lock()
			m.inputBuf = m.inputBuf[:0]
			m.mu.Unlock()
			ch <- text
			continue
		case 9: // Tab
			ch <- "\t"
			continue
		case '\033': // Escape sequence
			seq := make([]byte, 0, 8)
			// Read the rest of the escape sequence with timeout
			reader2 := bufio.NewReader(reader)
			for i := 0; i < 7; i++ {
				next, err := reader2.ReadByte()
				if err != nil {
					break
				}
				seq = append(seq, next)
				if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '~' {
					break
				}
			}
			seqStr := string(seq)
			if seqStr == "[Z" || seqStr == "[91;6u" {
				// Shift+Tab
				m.mu.Lock()
				if m.mode == ModeAgent {
					m.mode = ModePlan
				} else {
					m.mode = ModeAgent
				}
				m.addSystem("切换至 " + m.mode + " 模式")
				m.mu.Unlock()
				ch <- "\x01" // signal: mode changed, full render needed
			}
			continue
		default:
			m.mu.Lock()
			m.inputBuf = append(m.inputBuf, rune(b))
			m.mu.Unlock()
			ch <- "\x00"
		}
	}
}

// --- Input dispatch ---

func (m *Model) handleInput(text string) {
	text = strings.TrimSpace(text)
	if text == "" || text == "\x00" || text == "\t" {
		return
	}

	switch {
	case text == "/quit" || text == "/exit":
		m.running = false

	case text == "/help":
		m.addSystem("  /project <名>  /chars  /outline  /world  /write <n>  /export")
		m.addSystem("  /model  /mode agent|plan  /killchar <名>  /quit")
		m.addSystem("  Shift+Tab 切换 Agent/Plan 模式")

	case strings.HasPrefix(text, "/project"):
		name := strings.TrimSpace(strings.TrimPrefix(text, "/project"))
		m.cmdProject(name)

	case strings.HasPrefix(text, "/chars"):
		m.cmdListChars()

	case strings.HasPrefix(text, "/char "):
		name := strings.TrimSpace(strings.TrimPrefix(text, "/char "))
		m.cmdChar(name)

	case strings.HasPrefix(text, "/world"):
		m.cmdWorld()

	case strings.HasPrefix(text, "/outline"):
		m.cmdOutline()

	case strings.HasPrefix(text, "/write"):
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/write"))
		m.cmdWrite(arg)

	case strings.HasPrefix(text, "/model"):
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/model"))
		m.cmdModel(arg)

	case strings.HasPrefix(text, "/mode"):
		arg := strings.TrimSpace(strings.TrimPrefix(text, "/mode"))
		switch arg {
		case "plan":
			m.mode = ModePlan
		case "agent":
			m.mode = ModeAgent
		}
		m.addSystem("当前模式: " + m.mode)

	case strings.HasPrefix(text, "/killchar"):
		name := strings.TrimSpace(strings.TrimPrefix(text, "/killchar "))
		m.cmdKillChar(name)

	case strings.HasPrefix(text, "/export"):
		m.cmdExport()

	default:
		// Free-form — check write permission then AI route
		if m.mode == ModePlan && isWriteOp(text) {
			m.addWarning("Plan 模式不允许写操作。按 Shift+Tab 切换到 Agent 后重试。")
			return
		}
		m.addUser(text)
		m.cmdFreeForm(text)
	}
}

// --- Command handlers ---

func (m *Model) cmdProject(name string) {
	if name == "" {
		projs, _ := project.ListProjects(m.root)
		if len(projs) == 0 {
			m.addSystem("还没有项目。用法: /project <名称>")
			return
		}
		m.addSystem("项目列表: " + strings.Join(projs, ", "))
		return
	}
	dir := project.Dir(m.root, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := project.Init(m.root, name); err != nil {
			m.addError("创建项目失败: " + err.Error())
			return
		}
	}
	m.project = name
	m.addSystem("✓ 切换至项目: " + name)
}

func (m *Model) cmdListChars() {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	chars, err := project.ListCharacters(m.root, m.project)
	if err != nil || len(chars) == 0 {
		m.addSystem("还没有角色。使用 /char <角色名> 创建")
		return
	}
	for _, ch := range chars {
		icon := "🟢"
		if ch.Status == "deactivated" {
			icon = "⚫"
		}
		m.addSystem(fmt.Sprintf("  %s %s (%s)", icon, ch.Name, ch.Role))
	}
}

func (m *Model) cmdChar(name string) {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	if name == "" {
		m.addSystem("用法: /char <角色名>")
		return
	}
	id := project.CharacterID(name)
	existing, err := project.ReadCharacter(m.root, m.project, id)
	if err == nil {
		m.addSystem(fmt.Sprintf("📝 %s (%s) — %s", existing.Name, existing.Role, existing.Personality))
		return
	}
	ch := project.CharacterProfile{
		ID: id, Name: name, Role: "配角", Status: "active",
		Personality: "待设定", Background: "待设定", Motivation: "待设定",
	}
	project.WriteCharacter(m.root, m.project, ch)
	m.addSystem(fmt.Sprintf("✓ 角色「%s」已创建", name))
}

func (m *Model) cmdWorld() {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	w, _ := project.ReadWorld(m.root, m.project)
	m.addSystem(fmt.Sprintf("🌍 %v / %v | 力量体系: %v", w["genre"], w["sub_genre"], w["power_system"]))
}

func (m *Model) cmdOutline() {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	o, _ := project.ReadOutline(m.root, m.project)
	m.addSystem(fmt.Sprintf("📋 大纲 V%v | 已定稿: %v | 已写 %v 章", o["version"], o["finalized"], o["chapter_count"]))
}

func (m *Model) cmdWrite(arg string) {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	chNo := 1
	if arg != "" {
		if n, err := strconv.Atoi(arg); err == nil {
			chNo = n
		}
	}

	m.thinking = true
	m.thinkingMsg = fmt.Sprintf("正在续写第%d章...", chNo)

	taskID := fmt.Sprintf("%s-ch%03d-%d", m.project, chNo, time.Now().Unix())

	go func() {
		outline, _ := project.ReadOutline(m.root, m.project)
		world, _ := project.ReadWorld(m.root, m.project)

		skillName := "xuanhuan_writing"
		if g, ok := world["genre"].(string); ok && g != "" {
			skillName = g + "_writing"
		}

		ctx := context.Background()
		out, err := m.harness.RunStage(ctx, taskID, skillName, "content_generation", pipeline.StageInput{
			TrendData:      fmt.Sprintf("续写第%d章", chNo),
			ChapterOutline: fmt.Sprintf("第%d章大纲", chNo),
			ChapterNo:      chNo,
			NovelID:        m.project,
		})

		if err != nil {
			m.eventCh <- ChatEvent{StopSpinner: true, Err: err}
			return
		}

		m.mu.Lock()
		project.WriteChapter(m.root, m.project, chNo, out.Content)
		outline["chapter_count"] = chNo
		project.WriteOutline(m.root, m.project, outline)
		m.mu.Unlock()

		m.eventCh <- ChatEvent{
			StopSpinner: true,
			Line:        fmt.Sprintf("✓ 第%d章完成 (%d字)\n\n%s", chNo, len([]rune(out.Content)), trunc(out.Content, 500)),
		}
	}()
}

func (m *Model) cmdModel(arg string) {
	if arg == "" || arg == "list" {
		for _, p := range []string{"deepseek", "mimo", "minimax"} {
			_, err := m.harness.Router.GetClient(p)
			icon := "✗"
			if err == nil {
				icon = "✓"
			}
			label := model.ProviderLabels[p]
			if label == "" {
				label = p
			}
			m.addSystem(fmt.Sprintf("  %s %s — %s", icon, p, label))
		}
		return
	}
	parts := strings.SplitN(arg, " ", 2)
	if len(parts) == 2 && parts[0] == "switch" {
		m.activeModel = parts[1]
		m.addSystem(fmt.Sprintf("✓ 切换至 %s", parts[1]))
	}
}

func (m *Model) cmdKillChar(name string) {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	if name == "" {
		m.addSystem("用法: /killchar <角色名>")
		return
	}
	id := project.CharacterID(name)
	summary, err := project.DeactivateCharacter(m.root, m.project, id, "用户手动下线")
	if err != nil {
		m.addError(err.Error())
		return
	}
	m.addWarning(summary)
}

func (m *Model) cmdExport() {
	if m.project == "" {
		m.addSystem("请先选择项目: /project <名称>")
		return
	}
	all, err := project.ExportAll(m.root, m.project)
	if err != nil || all == "" {
		m.addSystem("还没有可导出的章节。使用 /write 续写")
		return
	}
	path := m.project + "-全书.txt"
	os.WriteFile(path, []byte(all), 0o644)
	m.addSystem(fmt.Sprintf("✓ 导出至 %s (%d字)", path, len([]rune(all))))
}

func (m *Model) cmdFreeForm(text string) {
	m.thinking = true
	m.thinkingMsg = "思考中..."

	go func() {
		client, _ := m.harness.Router.GetClient("deepseek")
		if client == nil {
			client, _ = m.harness.Router.GetClient("mimo")
		}
		if client == nil {
			m.eventCh <- ChatEvent{StopSpinner: true, Err: fmt.Errorf("没有可用的 AI 模型")}
			return
		}

		ctx := context.Background()
		reply, err := client.Generate(ctx, "你是小说创作助手。回答简洁，有网文创作经验。", text)
		if err != nil {
			m.eventCh <- ChatEvent{StopSpinner: true, Err: err}
			return
		}
		m.eventCh <- ChatEvent{StopSpinner: true, Line: reply}
	}()
}

// --- Chat helpers ---

// echoInput writes just the input area (3 lines) inline, without full render.
func (m *Model) echoInput() {
	// Move to bottom of terminal:
	//   \r = carriage return
	//   \033[K = clear to end of line
	//   \033[A = up one line

	// Input area is last 3 lines. Use cursor positioning to overwrite just those.
	// Simpler approach: use absolute cursor position based on termH.
	// But absolute positioning varies. Instead use relative: \033[s (save cursor)
	// + reposition + restore. Even simpler: just write \r\033[K for the single
	// input line, plus the mode label + hint if nothing else changed.
	// For now, print just the input line in-place:
	fmt.Fprintf(os.Stdout, "\r\033[K> %s█", string(m.inputBuf))
}

func (m *Model) addSystem(s string) {
	m.chatLines = append(m.chatLines, chatLine{Type: "system", Text: s})
}
func (m *Model) addUser(s string) {
	m.chatLines = append(m.chatLines, chatLine{Type: "user", Text: s})
}
func (m *Model) addAI(s string) {
	m.chatLines = append(m.chatLines, chatLine{Type: "ai", Text: s})
}
func (m *Model) addError(s string) {
	m.chatLines = append(m.chatLines, chatLine{Type: "error", Text: s})
}
func (m *Model) addWarning(s string) {
	m.chatLines = append(m.chatLines, chatLine{Type: "warning", Text: s})
}

func isWriteOp(text string) bool {
	for _, kw := range []string{"/write", "/char ", "/killchar", "写", "续写", "创建", "删除", "下线"} {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func trunc(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "\n...（后续内容已保存到 output/）"
}

// Terminal helpers

// Get terminal size using golang.org/x/term (cross-platform)
func getTermSize() (int, int, error) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80, 24, err
	}
	return w, h, nil
}

// Raw mode using golang.org/x/term (handles Windows + Unix)
var origState *term.State

func enableRawMode() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	origState = state
	return state, nil
}

func disableRawMode(_ *term.State) {
	if origState != nil {
		term.Restore(int(os.Stdin.Fd()), origState)
	}
}

func init() {
	// Suppress unused imports
	_ = audit.DefaultPolicy
}

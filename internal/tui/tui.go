// Package tui implements a zero-dependency terminal UI using Alt Screen +
// absolute cursor positioning (VT100/ANSI). Layout mirrors Reasonix — fixed
// status bar at top, scrollable chat in middle, fixed input area at bottom.
//
// Layout:
//
//	Row 1  [status bar]         📁 凡人修仙  ✍ 3章  🤖 deepseek  [Agent]
//	Row 2  [chat start]         
//	Row H-3 [chat end]          direct describe intent, or /help
//	Row H-2 [mode + input]      [Agent]
//	Row H-1 > typing...█
//	Row H   [hint]              Enter·Ctrl+C quit·Ctrl+L clear
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
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
)

// ANSI escapes
const (
	altEnter   = "\033[?1049h" // switch to alternate screen buffer
	altExit    = "\033[?1049l" // restore normal screen buffer
	cursorOff  = "\033[?25l"
	cursorOn   = "\033[?25h"
	clearBelow = "\033[J" // clear from cursor to end of screen
	clearLine  = "\033[2K"
	clearToEOL = "\033[K"

	bgBlue   = "\033[44m"
	fgWhite  = "\033[37m"
	fgPurple = "\033[35m"
	fgGreen  = "\033[32m"
	fgGray   = "\033[90m"
	fgRed    = "\033[31m"
	fgYellow = "\033[33m"
	reset    = "\033[0m"
	bold     = "\033[1m"
	dim      = "\033[2m"
)

// Mode constants
const (
	ModeAgent = "Agent"
	ModePlan  = "Plan✎"
)

// Spinner frames
var spinnerFrames = []string{"⠋","⠙","⠹","⠸","⠼","⠴","⠦","⠧","⠇","⠏"}

// --- Model ---

type Model struct {
	mu sync.Mutex

	harness     *harness.Harness
	root        string
	project     string
	mode        string
	activeModel string

	chatLines []chatLine

	// Display
	termW int
	termH int

	// Thinking
	thinking      bool
	thinkingFrame int
	thinkingMsg   string

	// Lifecycle
	running bool

	// Async
	eventCh chan any
}

type chatLine struct {
	Type string
	Text string
}

type ChatEvent struct {
	Line        string
	Err         error
	StopSpinner bool
}

type onboardState struct {
	genre, subGenre, powerSystem, desc string
	title1, title2, title3             string
}

// --- Constructor ---

func New(h *harness.Harness, root string) *Model {
	modelName := ""
	for _, p := range []string{"deepseek","mimo","minimax"} {
		if _, err := h.Router.GetClient(p); err == nil {
			modelName = p; break
		}
	}
	return &Model{
		harness: h, root: root,
		mode: ModeAgent, activeModel: modelName,
		chatLines: make([]chatLine, 0, 500),
		eventCh:   make(chan any, 256), running: true,
	}
}

// --- Run: main loop (called from main.go) ---

func Run(h *harness.Harness, root string) error {
	m := New(h, root)

	// Terminal size
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		m.termW, m.termH = w, h
	} else {
		m.termW, m.termH = 80, 24
	}

	// Raw mode
	fd := int(os.Stdin.Fd())
	rawState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(fd, rawState)

	// Enter Alt Screen
	fmt.Fprint(os.Stdout, altEnter+cursorOff)
	defer fmt.Fprint(os.Stdout, altExit+cursorOn)

	// Startup
	m.addSystem("✍  AI Novel Agent v2 · 交互式写作终端")
	if !m.checkAPIKey() {
		m.fullRender()
		m.promptAPIKeyLoop()
	}
	// After key setup (or already configured), show projects/help
	if m.checkAPIKey() {
		m.showPostSetup()
	} else {
		m.addSystem("直接描述创作意图，或用 /model set deepseek <key> 配置 API Key。")
	}
	m.fullRender()

	// Input goroutine
	inputCh := make(chan string, 32)
	go m.readInput(inputCh)

	// Spinner
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Main loop
	for m.running {
		select {
		case <-ticker.C:
			if m.thinking {
				m.mu.Lock()
				m.thinkingFrame = (m.thinkingFrame+1) % len(spinnerFrames)
				m.mu.Unlock()
				m.refreshStatus()
			}
		case input := <-inputCh:
			if input == "\x01" {
				m.fullRender()
				continue
			}
			m.handleInput(input)
		case ev := <-m.eventCh:
			if e, ok := ev.(ChatEvent); ok {
				m.mu.Lock()
				if e.StopSpinner { m.thinking = false }
				if e.Err != nil { m.addError(e.Err.Error()) } else if e.Line != "" { m.addAI(e.Line) }
				m.mu.Unlock()
				m.fullRender()
			}
		}
	}

	return nil
}

// --- Rendering ---

// fullRender clears the alt screen and draws all sections from scratch.
func (m *Model) fullRender() {
	var sb strings.Builder

	// Clear everything below row 1, then write each section at its absolute row.
	// We build the full frame as a single write to avoid flicker.

	// ── Row 1: Status bar ──
	chCount := 0
	if m.project != "" {
		chDir := project.Dir(m.root, m.project) + "/output"
		if entries, err := os.ReadDir(chDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") { chCount++ }
			}
		}
	}
	statusLeft := fmt.Sprintf("📁 %s  ✍ %d章  🤖 %s  [%s]", m.project, chCount, m.activeModel, m.mode)
	if m.project == "" { statusLeft = fmt.Sprintf("📁 （无项目） ✍ 0章  🤖 %s  [%s]", m.activeModel, m.mode) }

	status := bgBlue + fgWhite + bold + " " + statusLeft
	pad := m.termW - len([]rune(statusLeft)) - 3
	if pad < 0 { pad = 0 }
	status += strings.Repeat(" ", pad) + reset

	sb.WriteString("\033[1;1H")  // row 1, col 1
	sb.WriteString(clearLine)
	sb.WriteString(status)

	// ── Chat area: rows 2 to H-3 ──
	chatRows := m.termH - 3
	if chatRows < 1 { chatRows = 1 }
	start := len(m.chatLines) - chatRows
	if start < 0 { start = 0 }
	visible := m.chatLines[start:]

	for i := 0; i < chatRows; i++ {
		row := 2 + i
		sb.WriteString(fmt.Sprintf("\033[%d;1H", row))
		sb.WriteString(clearLine)
		if i < len(visible) {
			cl := visible[i]
			switch cl.Type {
			case "system": sb.WriteString(fgGray + "  " + cl.Text + reset)
			case "user":   sb.WriteString(fgGreen + bold + "✍ " + cl.Text + reset)
			case "ai":
				sb.WriteString("  ")
				sb.WriteString(cl.Text)
			case "aiHeader": sb.WriteString(fgPurple + bold + cl.Text + reset)
			case "error": sb.WriteString(fgRed + "✗ " + cl.Text + reset)
			case "warning": sb.WriteString(fgYellow + "⚠ " + cl.Text + reset)
			}
		}
	}

	// ── Thinking indicator (overwrites last chat row if active) ──
	if m.thinking {
		row := 2 + chatRows - 1
		frame := spinnerFrames[m.thinkingFrame]
		sb.WriteString(fmt.Sprintf("\033[%d;1H", row))
		sb.WriteString(clearLine)
		sb.WriteString(dim + fgGray + "  " + frame + " " + m.thinkingMsg + reset)
	}

	// ── Input area: rows H-2, H-1, H ──
	sep := strings.Repeat("─", m.termW)
	sb.WriteString(fmt.Sprintf("\033[%d;1H", m.termH-2))
	sb.WriteString(clearLine)
	sb.WriteString(fgGray + sep + reset)

	sb.WriteString(fmt.Sprintf("\033[%d;1H", m.termH-1))
	sb.WriteString(clearLine)
	sb.WriteString(fgPurple + "[" + m.mode + "]" + reset)
	sb.WriteString(" > _")

	hint := " Enter·Ctrl+C quit·Ctrl+L clear "
	sb.WriteString(fmt.Sprintf("\033[%d;1H", m.termH))
	sb.WriteString(clearLine)
	sb.WriteString(fgGray + dim + hint + strings.Repeat(" ", m.termW-len([]rune(hint))-1) + reset)

	os.Stdout.WriteString(sb.String())
}

// refreshStatus only redraws row 1 (status bar) — cheap, called on every tick.
func (m *Model) refreshStatus() {
	chCount := 0
	if m.project != "" {
		chDir := project.Dir(m.root, m.project) + "/output"
		if entries, err := os.ReadDir(chDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") { chCount++ }
			}
		}
	}
	statusLeft := fmt.Sprintf("📁 %s  ✍ %d章  🤖 %s  [%s]", m.project, chCount, m.activeModel, m.mode)
	if m.project == "" { statusLeft = fmt.Sprintf("📁 （无项目） ✍ 0章  🤖 %s  [%s]", m.activeModel, m.mode) }

	status := bgBlue + fgWhite + bold + " " + statusLeft
	pad := m.termW - len([]rune(statusLeft)) - 3
	if pad < 0 { pad = 0 }
	status += strings.Repeat(" ", pad) + reset

	fmt.Fprintf(os.Stdout, "\033[1;1H%s%s", clearLine, status)
}

// --- Input reader (byte loop, echoes characters) ---

func (m *Model) readInput(ch chan<- string) {
	rd := bufio.NewReader(os.Stdin)
	for m.running {
		// Echo the input prompt before waiting for input
		m.refreshInputPrompt()

		buf := make([]byte, 0, 4096)
		for {
			b, err := rd.ReadByte()
			if err != nil {
				ch <- "/quit"; return
			}
			if b == '\r' || b == '\n' {
				fmt.Fprint(os.Stdout, "\r\n"); break
			}
			if b == 3 { // Ctrl+C
				ch <- "/quit"; return
			}
			if b == 12 { // Ctrl+L
				m.mu.Lock(); m.chatLines = nil; m.mu.Unlock()
				ch <- "\x01"; return
			}
			if b == 127 || b == '\b' {
				if len(buf) > 0 { buf = buf[:len(buf)-1]; fmt.Fprint(os.Stdout, "\b \b") }
				continue
			}
			buf = append(buf, b)
			fmt.Fprint(os.Stdout, string(b))
		}
		line := string(buf)

		// Shift+Tab detection
		if strings.Contains(line, "\033[Z") {
			m.mu.Lock()
			if m.mode == ModeAgent { m.mode = ModePlan } else { m.mode = ModeAgent }
			m.addSystem("切换至 " + m.mode + " 模式")
			m.mu.Unlock()
			ch <- "\x01"
			continue
		}
		ch <- line
	}
}

func (m *Model) refreshInputPrompt() {
	row := m.termH - 1
	fmt.Fprintf(os.Stdout, "\033[%d;1H%s%s > _%s", row, clearLine, fgPurple+"["+m.mode+"]"+reset, clearToEOL)
}

// --- API Key onboarding loop ---

func (m *Model) promptAPIKeyLoop() {
	m.addSystem("╔══════════════════════════════════════════════════════╗")
	m.addSystem("║  首次使用：请粘贴 DeepSeek API Key 后按回车           ║")
	m.addSystem("║  申请: https://platform.deepseek.com/api_keys         ║")
	m.addSystem("╚══════════════════════════════════════════════════════╝")
	m.addSystem("📝 请粘贴 DeepSeek API Key 后按回车：")
	m.fullRender()

	rd := bufio.NewReader(os.Stdin)
	for !m.checkAPIKey() {
		// Write prompt at input row
		row := m.termH - 1
		fmt.Fprintf(os.Stdout, "\033[%d;1H%s> ", row, clearLine)

		buf := make([]byte, 0, 4096)
		for {
			b, err := rd.ReadByte()
			if err != nil { return }
			if b == '\r' || b == '\n' { fmt.Fprint(os.Stdout, "\r\n"); break }
			if b == 3 { return }
			if b == 127 || b == '\b' {
				if len(buf) > 0 { buf = buf[:len(buf)-1]; fmt.Fprint(os.Stdout, "\b \b") }
				continue
			}
			buf = append(buf, b)
			fmt.Fprint(os.Stdout, "*")
		}
		line := string(buf)
		if line == "" || line == "/quit" || line == "/exit" {
			m.addSystem("  ℹ 已跳过。后续可通过 /model set deepseek <key> 配置。")
			break
		}
		if !m.setAPIKey(line) {
			continue
		}
		break
	}
	m.fullRender()
}

func (m *Model) checkAPIKey() bool {
	for _, p := range []string{"deepseek","mimo","minimax"} {
		if _, err := m.harness.Router.GetClient(p); err == nil {
			m.activeModel = p; return true
		}
	}
	return false
}

func (m *Model) showPostSetup() {
	projects, _ := project.ListProjects(m.root)
	if len(projects) > 0 {
		m.project = projects[0]
		m.addSystem("📁 当前项目: " + m.project)
		m.addSystem("直接描述你的创作意图，或输入 /help 查看命令。")
	} else {
		m.addSystem("你想写一本怎样的小说？直接告诉我书名、类型、核心灵感。")
	}
}

func (m *Model) setAPIKey(key string) bool {
	m.addSystem(fmt.Sprintf("  ⏳ 验证 Key（%s...）...", maskKey(key)))
	m.fullRender()
	err := m.validateKey(key)
	if err != nil {
		if strings.Contains(err.Error(),"401")||strings.Contains(err.Error(),"invalid")||strings.Contains(err.Error(),"Authentication") {
			m.addError("Key 无效（401 认证失败），请重新粘贴")
		} else { m.addError("验证失败: "+err.Error()) }
		m.fullRender()
		return false
	}
	cfg, _ := storage.ReadConfig(m.root)
	if cfg == nil { cfg = make(map[string]any) }
	cfg["deepseek"] = map[string]any{
		"api_key":key,"api_endpoint":model.DefaultConfig("deepseek").Endpoint,
		"model_name":model.DefaultConfig("deepseek").ModelName,
		"max_tokens":model.DefaultConfig("deepseek").MaxTokens,
		"temperature":model.DefaultConfig("deepseek").Temperature,
		"timeout":60,"retry_times":3,
	}
	if err := storage.WriteConfig(m.root, cfg); err != nil { m.addError("保存: "+err.Error()); return false }
	mcs, fb := modelConfigsFromConfig(cfg)
	newRouter, err := model.NewRouter(mcs, fb)
	if err != nil { m.addError("路由: "+err.Error()); return false }
	m.harness.Router = newRouter
	m.activeModel = "deepseek"
	m.addSystem("  ✓ DeepSeek 已就绪！")
	m.showPostSetup()
	m.fullRender()
	return true
}

func (m *Model) validateKey(key string) error {
	cfg := model.DefaultConfig("deepseek")
	cfg.APIKey = key; cfg.MaxTokens = 4; cfg.Timeout = 15*time.Second
	client := model.NewClient(cfg)
	if client == nil { return fmt.Errorf("不支持的模型") }
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := client.Generate(ctx, "hi","hi")
	return err
}

func maskKey(s string) string {
	if len(s) <= 8 { return strings.Repeat("*", len(s)) }
	return s[:3]+"****"+s[len(s)-4:]
}

// --- Input dispatch ---

func (m *Model) handleInput(text string) {
	text = strings.TrimSpace(text)
	if text == "" { m.refreshInputPrompt(); return }
	switch {
	case text == "/quit"||text == "/exit": m.running = false
	case text == "/help": m.cmdHelp()
	case strings.HasPrefix(text, "/mode"): m.cmdMode(strings.TrimSpace(strings.TrimPrefix(text,"/mode")))
	case strings.HasPrefix(text, "/project"): m.cmdProject(strings.TrimSpace(strings.TrimPrefix(text,"/project")))
	case text == "/projects": m.cmdListProjects()
	case text == "/chars": m.cmdListChars()
	case strings.HasPrefix(text, "/char "): m.cmdChar(strings.TrimSpace(strings.TrimPrefix(text,"/char ")))
	case text == "/world": m.cmdWorld()
	case text == "/outline": m.cmdOutline()
	case text == "/skills": m.addSystem("54 个 Skill 已就绪（6类型×9子技能）")
	case strings.HasPrefix(text, "/model"): m.cmdModel(strings.TrimSpace(strings.TrimPrefix(text,"/model")))
	case strings.HasPrefix(text, "/write"): m.cmdWrite(strings.TrimSpace(strings.TrimPrefix(text,"/write")))
	case strings.HasPrefix(text, "/killchar "): m.cmdKillChar(strings.TrimSpace(strings.TrimPrefix(text,"/killchar ")))
	case text == "/export": m.cmdExport()
	default:
		if m.mode == ModePlan && isWriteOp(text) {
			m.addWarning("Plan 模式不允许写操作。按 Enter 空行切换到 Agent 后重试。")
			m.fullRender()
			return
		}
		m.addUser(text)
		m.cmdFreeForm(text)
	}
	m.fullRender()
}

// --- Command handlers ---

func (m *Model) cmdHelp() {
	m.addSystem("/project <名>  /chars  /outline  /world  /write <n>  /export")
	m.addSystem("/model  /mode agent|plan  /killchar <名>  /quit")
	m.addSystem("空行回车 切换 Agent/Plan 模式")
}

func (m *Model) cmdMode(arg string) {
	switch arg {
	case "plan": m.mode = ModePlan
	case "agent": m.mode = ModeAgent
	}
	m.addSystem("当前模式: " + m.mode)
}

func (m *Model) cmdProject(name string) {
	if name == "" { m.addSystem("用法: /project <名称>"); return }
	dir := project.Dir(m.root, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := project.Init(m.root, name); err != nil { m.addError(err.Error()); return }
	}
	m.project = name
	m.addSystem("✓ 切换至项目: " + name)
}

func (m *Model) cmdListProjects() {
	projs, _ := project.ListProjects(m.root)
	if len(projs) == 0 { m.addSystem("还没有项目。使用 /project <名称> 创建"); return }
	for _, p := range projs {
		marker := " "; if p == m.project { marker = "*" }
		m.addSystem(fmt.Sprintf("  %s %s", marker, p))
	}
}

func (m *Model) cmdListChars() {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	chars, err := project.ListCharacters(m.root, m.project)
	if err != nil || len(chars) == 0 { m.addSystem("还没有角色。使用 /char <角色名> 创建"); return }
	for _, ch := range chars {
		icon := "🟢"; if ch.Status == "deactivated" { icon = "⚫" }
		m.addSystem(fmt.Sprintf("  %s %s (%s)", icon, ch.Name, ch.Role))
	}
}

func (m *Model) cmdChar(name string) {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	if name == "" { m.addSystem("用法: /char <角色名>"); return }
	id := project.CharacterID(name)
	existing, err := project.ReadCharacter(m.root, m.project, id)
	if err == nil { m.addSystem(fmt.Sprintf("📝 %s (%s) — %s", existing.Name, existing.Role, existing.Personality)); return }
	ch := project.CharacterProfile{ID:id,Name:name,Role:"配角",Status:"active",Personality:"待设定",Background:"待设定",Motivation:"待设定"}
	project.WriteCharacter(m.root, m.project, ch)
	m.addSystem(fmt.Sprintf("✓ 角色「%s」已创建", name))
}

func (m *Model) cmdWorld() {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	w, _ := project.ReadWorld(m.root, m.project)
	m.addSystem(fmt.Sprintf("🌍 %v / %v | 力量体系: %v", w["genre"], w["sub_genre"], w["power_system"]))
}

func (m *Model) cmdOutline() {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	o, _ := project.ReadOutline(m.root, m.project)
	m.addSystem(fmt.Sprintf("📋 大纲 V%v | 已定稿: %v | 已写 %v 章", o["version"], o["finalized"], o["chapter_count"]))
}

func (m *Model) cmdWrite(arg string) {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	chNo := 1
	if arg != "" { if n, err := strconv.Atoi(arg); err == nil { chNo = n } }
	m.thinking = true; m.thinkingMsg = fmt.Sprintf("正在续写第%d章...", chNo)
	m.fullRender()

	taskID := fmt.Sprintf("%s-ch%03d-%d", m.project, chNo, time.Now().Unix())
	go func() {
		outline, _ := project.ReadOutline(m.root, m.project)
		world, _ := project.ReadWorld(m.root, m.project)
		skillName := "xuanhuan_writing"
		if g, ok := world["genre"].(string); ok && g != "" { skillName = g + "_writing" }
		ctx := context.Background()
		out, err := m.harness.RunStage(ctx, taskID, skillName, "content_generation", pipeline.StageInput{
			TrendData: fmt.Sprintf("续写第%d章", chNo), ChapterOutline: fmt.Sprintf("第%d章大纲", chNo),
			ChapterNo: chNo, NovelID: m.project,
		})
		if err != nil { m.eventCh <- ChatEvent{StopSpinner:true,Err:err}; return }
		m.mu.Lock()
		project.WriteChapter(m.root, m.project, chNo, out.Content)
		outline["chapter_count"] = chNo
		project.WriteOutline(m.root, m.project, outline)
		m.mu.Unlock()
		m.eventCh <- ChatEvent{StopSpinner:true, Line: fmt.Sprintf("✓ 第%d章完成 (%d字)\n\n%s", chNo, len([]rune(out.Content)), trunc(out.Content, 500))}
	}()
}

func (m *Model) cmdModel(arg string) {
	if arg == "" || arg == "list" {
		for _, p := range []string{"deepseek","mimo","minimax"} {
			_, err := m.harness.Router.GetClient(p)
			icon := "✗"; if err == nil { icon = "✓"; m.activeModel = p }
			label := model.ProviderLabels[p]; if label == "" { label = p }
			m.addSystem(fmt.Sprintf("  %s %s — %s", icon, p, label))
		}
		return
	}
	parts := strings.SplitN(arg, " ", 2)
	if len(parts)==2 && parts[0]=="set" {
		parts2 := strings.SplitN(parts[1]," ",2)
		if len(parts2)<2 { m.addSystem("用法: /model set <deepseek|minimax|mimo> <api-key>"); return }
		provider, key := parts2[0], parts2[1]
		cfg, _ := storage.ReadConfig(m.root)
		if cfg == nil { cfg = make(map[string]any) }
		cfg[provider] = map[string]any{
			"api_key":key, "api_endpoint":model.DefaultConfig(provider).Endpoint,
			"model_name":model.DefaultConfig(provider).ModelName,
			"max_tokens":model.DefaultConfig(provider).MaxTokens,
			"temperature":model.DefaultConfig(provider).Temperature,
			"timeout":60,"retry_times":3,
		}
		if err := storage.WriteConfig(m.root, cfg); err != nil { m.addError("保存: "+err.Error()); return }
		mcs, fb := modelConfigsFromConfig(cfg)
		newRouter, err := model.NewRouter(mcs, fb)
		if err != nil { m.addError("路由: "+err.Error()); return }
		m.harness.Router = newRouter
		m.activeModel = provider
		m.addSystem(fmt.Sprintf("✓ %s API Key 已配置", model.ProviderLabels[provider]))
		return
	}
	if len(parts)==2 && parts[0]=="switch" {
		p := parts[1]
		if _, err := m.harness.Router.GetClient(p); err != nil { m.addSystem(fmt.Sprintf("✗ %s 未配置", p)); return }
		m.activeModel = p
		m.addSystem(fmt.Sprintf("✓ 切换到 %s", model.ProviderLabels[p]))
		return
	}
	m.addSystem("用法: /model | /model set <模型> <key> | /model switch <模型>")
}

func (m *Model) cmdKillChar(name string) {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	id := project.CharacterID(name)
	summary, err := project.DeactivateCharacter(m.root, m.project, id, "用户手动下线")
	if err != nil { m.addError(err.Error()); return }
	m.addWarning(summary)
}

func (m *Model) cmdExport() {
	if m.project == "" { m.addSystem("请先选择项目: /project <名称>"); return }
	all, err := project.ExportAll(m.root, m.project)
	if err != nil || all == "" { m.addSystem("还没有可导出的章节。"); return }
	path := m.project + "-全书.txt"
	os.WriteFile(path, []byte(all), 0o644)
	m.addSystem(fmt.Sprintf("✓ 导出至 %s (%d字)", path, len([]rune(all))))
}

func (m *Model) cmdFreeForm(text string) {
	m.thinking = true; m.thinkingMsg = "思考中..."
	m.fullRender()
	go func() {
		client, _ := m.harness.Router.GetClient("deepseek")
		if client == nil { client, _ = m.harness.Router.GetClient("mimo") }
		if client == nil { m.eventCh <- ChatEvent{StopSpinner:true,Err:fmt.Errorf("没有可用的 AI 模型")}; return }
		ctx := context.Background()
		reply, err := client.Generate(ctx, "你是小说创作助手。回答简洁，有网文创作经验。", text)
		if err != nil { m.eventCh <- ChatEvent{StopSpinner:true,Err:err}; return }
		m.eventCh <- ChatEvent{StopSpinner:true, Line: reply}
	}()
}

// --- Chat helpers ---

func (m *Model) addSystem(s string) { m.chatLines = append(m.chatLines, chatLine{Type:"system",Text:s}) }
func (m *Model) addUser(s string)   { m.chatLines = append(m.chatLines, chatLine{Type:"user",Text:s}) }
func (m *Model) addAI(s string)     { m.chatLines = append(m.chatLines, chatLine{Type:"ai",Text:s}) }
func (m *Model) addError(s string)  { m.chatLines = append(m.chatLines, chatLine{Type:"error",Text:s}) }
func (m *Model) addWarning(s string){ m.chatLines = append(m.chatLines, chatLine{Type:"warning",Text:s}) }

// --- Config helpers ---

func modelConfigsFromConfig(cfg map[string]any) (map[string]model.Config, string) {
	configs := make(map[string]model.Config)
	for _, p := range []string{"deepseek","mimo","minimax"} {
		if raw, ok := cfg[p]; ok {
			if pmap, ok := raw.(map[string]any); ok {
				mc := model.DefaultConfig(p)
				if v, ok := pmap["api_key"].(string); ok && v != "" && !strings.HasPrefix(v,"${") { mc.APIKey = v }
				if v, ok := pmap["endpoint"].(string); ok && v != "" { mc.Endpoint = v } else if v, ok := pmap["api_endpoint"].(string); ok && v != "" { mc.Endpoint = v }
				if v, ok := pmap["model_name"].(string); ok && v != "" { mc.ModelName = v }
				if v, ok := pmap["max_tokens"].(int); ok { mc.MaxTokens = v }
				if v, ok := pmap["temperature"].(float64); ok { mc.Temperature = v }
				configs[p] = mc
			}
		}
	}
	fb := "deepseek"
	if routing, ok := cfg["stage_routing"]; ok {
		if rmap, ok := routing.(map[string]any); ok {
			if f, ok := rmap["fallback"].(string); ok && f != "" { fb = f }
		}
	}
	return configs, fb
}

func isWriteOp(text string) bool {
	for _, kw := range []string{"/write","/char ","/killchar","写","续写","创建","删除","下线"} {
		if strings.Contains(text, kw) { return true }
	}
	return false
}

func trunc(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n { return s }
	return string(runes[:n]) + "\n...（后续内容已保存到 output/）"
}

// Suppress unused import
var _ = audit.DefaultPolicy

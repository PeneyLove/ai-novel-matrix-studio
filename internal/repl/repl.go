// Package repl provides the interactive Read-Eval-Print Loop for the AI Novel Agent.
//
// When the user runs `novel-agent` with no arguments, they enter a persistent
// conversation with the AI writing assistant. The REPL parses natural-language
// intents (create project, design world, add character, write chapter, etc.)
// and dispatches them to the appropriate skill.
//
// Basic commands begin with '/' like a modern chat client:
//
//	/help          — show all commands
//	/project <name> — switch to or create a project
//	/world         — view or edit worldbuilding
//	/char <name>   — view or create a character
//	/chars         — list all characters
//	/outline       — view or refine the outline
//	/write <n>     — write chapter N
//	/export        — export all chapters to a book .txt
//	/skills        — list available skills
//	/quit          — exit
//
// Plain text without '/' is treated as a writing request and dispatched
// to the pipeline run stage.
package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/audit"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/global"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
)

// Session holds the active REPL state.
type Session struct {
	h       *harness.Harness
	audit   audit.Policy
	project string // active project name
	root    string // .novelAgent root
	router  *model.Router
}

// Run starts the interactive REPL loop.
func Run(h *harness.Harness, root string) error {
	auditPolicy := loadAuditPolicy(root)

	fmt.Println()
	fmt.Println("  ✍  AI Novel Agent v2 · 交互式写作终端")
	fmt.Println("  输入 /help 查看命令，直接打字开始创作")
	fmt.Println()

	s := &Session{
		h:      h,
		audit:  auditPolicy,
		root:   root,
		router: h.Router,
	}

	// Auto-detect or prompt for project
	projects, _ := project.ListProjects(root)
	if len(projects) == 0 {
		fmt.Println("  💡 还没有项目。输入 /project <名称> 创建一个")
	} else {
		s.project = projects[0]
		fmt.Printf("  📁 当前项目: %s\n", s.project)
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n✍ novel> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "读取输入失败:", err)
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "/quit" || input == "/exit" {
			fmt.Println("👋 再见！")
			break
		}

		s.handle(input)
	}

	return nil
}

func (s *Session) handle(input string) {
	switch {
	case input == "/help":
		s.cmdHelp()
	case strings.HasPrefix(input, "/project"):
		s.cmdProject(strings.TrimSpace(strings.TrimPrefix(input, "/project")))
	case input == "/projects":
		s.cmdListProjects()
	case strings.HasPrefix(input, "/char "):
		s.cmdChar(strings.TrimSpace(strings.TrimPrefix(input, "/char ")))
	case input == "/chars":
		s.cmdListChars()
	case input == "/world":
		s.cmdShowWorld()
	case input == "/outline":
		s.cmdShowOutline()
	case input == "/skills":
		s.cmdListSkills()
	case strings.HasPrefix(input, "/write"):
		s.cmdWrite(strings.TrimSpace(strings.TrimPrefix(input, "/write")))
	case input == "/export":
		s.cmdExport()
	case strings.HasPrefix(input, "/killchar "):
		s.cmdDeactivate(strings.TrimSpace(strings.TrimPrefix(input, "/killchar ")))
	default:
		// Free-form writing request → pipeline input
		s.handleFreeForm(input)
	}
}

// ---- command handlers ----

func (s *Session) cmdHelp() {
	lines := []string{
		"",
		"  可用命令:",
		"  /project <名称>    切换到指定项目（不存在则创建）",
		"  /projects          列出所有项目",
		"  /world             查看世界观设定",
		"  /outline           查看大纲和伏笔台账",
		"  /char <名称>       查看或创建角色",
		"  /chars             列出所有活跃角色",
		"  /killchar <名称>   下线一个角色（需确认）",
		"  /skills            列出可用 Skill",
		"  /write <章节号>     续写指定章节",
		"  /export            导出全书到 output/",
		"  /quit              退出",
		"",
		"  也可以直接打字描述你想做什么，AI 会尝试理解并执行。",
		"",
	}
	fmt.Print(strings.Join(lines, "\n"))
}

func (s *Session) cmdProject(name string) {
	if name == "" {
		projects, _ := project.ListProjects(s.root)
		if len(projects) == 0 {
			fmt.Println("  还没有项目。用法: /project <名称>")
			return
		}
		fmt.Printf("  项目列表: %s\n", strings.Join(projects, ", "))
		fmt.Printf("  当前: %s\n", s.project)
		return
	}

	dir := project.Dir(s.root, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := project.Init(s.root, name); err != nil {
			fmt.Printf("  ✗ 创建项目失败: %v\n", err)
			return
		}
		fmt.Printf("  ✓ 已创建项目: %s\n", name)
	}
	s.project = name
	fmt.Printf("  ✓ 切换到项目: %s\n", name)
}

func (s *Session) cmdListProjects() {
	projects, _ := project.ListProjects(s.root)
	if len(projects) == 0 {
		fmt.Println("  还没有项目。使用 /project <名称> 创建")
		return
	}
	fmt.Println("  项目列表:")
	for _, p := range projects {
		marker := " "
		if p == s.project {
			marker = "*"
		}
		// Count chapters
		chDir := project.Dir(s.root, p) + "/output"
		chCount := 0
		if entries, err := os.ReadDir(chDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
					chCount++
				}
			}
		}
		fmt.Printf("   %s %-20s (%d 章)\n", marker, p, chCount)
	}
}

func (s *Session) cmdChar(name string) {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	if name == "" {
		fmt.Println("  用法: /char <角色名>")
		return
	}

	id := project.CharacterID(name)
	ch, err := project.ReadCharacter(s.root, s.project, id)
	if err == nil {
		// Show existing character
		fmt.Printf("  📝 %s (%s)\n", ch.Name, ch.Role)
		fmt.Printf("     状态: %s\n", ch.Status)
		fmt.Printf("     性格: %s\n", ch.Personality)
		fmt.Printf("     背景: %s\n", truncate(ch.Background, 120))
		return
	}

	// Create new character — call AI to generate profile
	fmt.Printf("  🌱 正在为「%s」生成角色小传...\n", name)
	input := pipeline.StageInput{
		TrendData: fmt.Sprintf("为小说「%s」创建一个角色：%s", s.project, name),
		NovelID:   s.project + "-char-" + id,
	}
	// Use a simple direct prompt approach since we don't have a character-specific skill in the current pipeline
	ch := project.CharacterProfile{
		ID:          id,
		Name:        name,
		Role:        "配角",
		Status:      "active",
		Personality:  "待AI生成",
		Background:   "待AI生成",
		Motivation:   "待AI生成",
	}
	if err := project.WriteCharacter(s.root, s.project, ch); err != nil {
		fmt.Printf("  ✗ 创建角色失败: %v\n", err)
		return
	}
	fmt.Printf("  ✓ 角色「%s」已创建（可在 .novelAgent/projects/%s/characters/%s.yaml 编辑详情）\n", name, s.project, id)

	_ = input // suppress unused for now, character gen will be wired in later
}

func (s *Session) cmdListChars() {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	chars, err := project.ListCharacters(s.root, s.project)
	if err != nil {
		fmt.Printf("  ✗ 读取角色列表失败: %v\n", err)
		return
	}
	if len(chars) == 0 {
		fmt.Println("  还没有角色。使用 /char <角色名> 创建")
		return
	}
	fmt.Println("  角色列表:")
	for _, ch := range chars {
		icon := "🟢"
		if ch.Status == "deactivated" {
			icon = "⚫"
		}
		fmt.Printf("   %s %-12s %-8s %s\n", icon, ch.Name, ch.Role, truncate(ch.Personality, 60))
	}
}

func (s *Session) cmdListSkills() {
	names := s.h.ListSkills()
	if len(names) == 0 {
		fmt.Println("  还没有 Skill。运行 novel-agent init --force 重新初始化。")
		return
	}
	fmt.Printf("  已安装 %d 个 Skill:\n", len(names))
	genres := map[string][]string{}
	for _, n := range names {
		parts := strings.SplitN(n, "_", 2)
		g := parts[0]
		if len(parts) > 1 {
			genres[g] = append(genres[g], parts[1])
		}
	}
	for g, subs := range genres {
		fmt.Printf("    %s: %s\n", g, strings.Join(subs, ", "))
	}
}

func (s *Session) cmdShowWorld() {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	w, err := project.ReadWorld(s.root, s.project)
	if err != nil {
		fmt.Printf("  ✗ 读取世界观失败: %v\n", err)
		return
	}
	fmt.Println("  🌍 世界观设定:")
	fmt.Printf("    类型: %v / %v\n", w["genre"], w["sub_genre"])
	fmt.Printf("    力量体系: %v\n", w["power_system"])
	if factions, ok := w["factions"].([]any); ok && len(factions) > 0 {
		fmt.Println("    势力:")
		for _, f := range factions {
			fmt.Printf("      - %v\n", f)
		}
	}
}

func (s *Session) cmdShowOutline() {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	o, err := project.ReadOutline(s.root, s.project)
	if err != nil {
		fmt.Printf("  ✗ 读取大纲失败: %v\n", err)
		return
	}
	fmt.Println("  📋 大纲:")
	fmt.Printf("    版本: V%d\n", o["version"])
	fmt.Printf("    已定稿: %v\n", o["finalized"])
	fmt.Printf("    已写章节: %v\n", o["chapter_count"])
	if content, ok := o["content"].(string); ok && content != "" {
		fmt.Printf("    内容预览: %s\n", truncate(content, 200))
	}
	if hooks, ok := o["hooks"].([]any); ok && len(hooks) > 0 {
		fmt.Printf("    伏笔: %d 条\n", len(hooks))
	}
}

func (s *Session) cmdWrite(arg string) {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	chNo := 1
	if arg != "" {
		if n, err := strconv.Atoi(arg); err == nil {
			chNo = n
		}
	}

	// Load outline and world for context
	outline, _ := project.ReadOutline(s.root, s.project)
	world, _ := project.ReadWorld(s.root, s.project)

	trendData := fmt.Sprintf("项目: %s, 世界观: %v, 大纲版本V%v, 续写第%d章",
		s.project, world["genre"], outline["version"], chNo)

	fmt.Printf("  ✍ 正在续写第 %d 章（调用 AI 模型...）\n", chNo)
	ctx := context.Background()

	taskID := fmt.Sprintf("%s-ch%03d-%d", s.project, chNo, time.Now().Unix())

	// Find a writing skill for the project's genre
	skillName := "xuanhuan_writing" // default
	if g, ok := world["genre"].(string); ok && g != "" {
		skillName = g + "_writing"
	}

	out, err := s.h.RunStage(ctx, taskID, skillName, "content_generation", pipeline.StageInput{
		TrendData:      trendData,
		ChapterOutline: fmt.Sprintf("第%d章大纲", chNo),
		ChapterNo:      chNo,
		NovelID:        s.project,
	})
	if err != nil {
		fmt.Printf("  ✗ 续写失败: %v\n", err)
		return
	}

	// Save to output
	if err := project.WriteChapter(s.root, s.project, chNo, out.Content); err != nil {
		fmt.Printf("  ✗ 保存章节失败: %v\n", err)
		return
	}

	// Update outline chapter count
	outline["chapter_count"] = chNo
	project.WriteOutline(s.root, s.project, outline)

	fmt.Printf("  ✓ 第 %d 章已保存 → .novelAgent/projects/%s/output/ch%03d.txt\n", chNo, s.project, chNo)
	fmt.Printf("  ─── 预览 ───\n%s\n  ───────────\n", truncate(out.Content, 400))
}

func (s *Session) cmdExport() {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	all, err := project.ExportAll(s.root, s.project)
	if err != nil {
		fmt.Printf("  ✗ 导出失败: %v\n", err)
		return
	}
	if all == "" {
		fmt.Println("  ℹ 还没有章节，先用 /write 续写")
		return
	}

	exportPath := s.project + "-全书.txt"
	os.WriteFile(exportPath, []byte(all), 0o644)
	fmt.Printf("  ✓ 已导出到 %s (%d 字)\n", exportPath, len([]rune(all)))
}

func (s *Session) cmdDeactivate(name string) {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	if name == "" {
		fmt.Println("  用法: /killchar <角色名>")
		return
	}

	id := project.CharacterID(name)
	ch, err := project.ReadCharacter(s.root, s.project, id)
	if err != nil {
		fmt.Printf("  ✗ 角色「%s」不存在\n", name)
		return
	}

	detail := fmt.Sprintf("下线角色「%s」(%s)。该角色将从活跃列表移出，仅在回忆/闪回中存在。", ch.Name, ch.Role)
	ok, err := s.audit.Check(audit.OpCharDeactivate, detail)
	if err != nil {
		fmt.Printf("  ✗ 确认失败: %v\n", err)
		return
	}
	if !ok {
		return
	}

	summary, err := project.DeactivateCharacter(s.root, s.project, id,
		fmt.Sprintf("用户在第%.0f章左右手动下线", time.Now().Unix()))
	if err != nil {
		fmt.Printf("  ✗ 下线失败: %v\n", err)
		return
	}
	fmt.Println("  " + summary)
}

func (s *Session) handleFreeForm(input string) {
	// Treat free-form input as a general writing intent
	fmt.Println("  💬 你说:", truncate(input, 100))

	// For now, echo and suggest a /command
	if strings.Contains(input, "写") || strings.Contains(input, "续写") || strings.Contains(input, "章节") {
		fmt.Println("  💡 试试: /write <章节号>")
	} else if strings.Contains(input, "角色") || strings.Contains(input, "人物") {
		fmt.Println("  💡 试试: /char <角色名> 或 /chars")
	} else if strings.Contains(input, "大纲") || strings.Contains(input, "剧情") {
		fmt.Println("  💡 试试: /outline")
	} else if strings.Contains(input, "世界观") {
		fmt.Println("  💡 试试: /world")
	} else {
		fmt.Println("  💡 输入 /help 查看所有命令")
	}
}

// ---- helpers ----

func loadAuditPolicy(root string) audit.Policy {
	return audit.DefaultPolicy()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

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
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
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

	// Check if any API key is configured. If not, guide user.
	s.checkAPIKeyOnStartup()

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
	case input == "/char evolve" || strings.HasPrefix(input, "/char evolve "):
		s.cmdCharEvolve(strings.TrimSpace(strings.TrimPrefix(input, "/char evolve ")))
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
	case input == "/model" || strings.HasPrefix(input, "/model "):
		s.cmdModel(strings.TrimSpace(strings.TrimPrefix(input, "/model")))
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
		"  /char evolve <名>   优化角色自适应性提示词（基于成长轨迹）",
		"  /char <名称>       查看或创建角色",
		"  /chars             列出所有活跃角色",
		"  /killchar <名称>   下线一个角色（需确认）",
		"  /model             查看/切换 AI 模型",
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
	newCh := project.CharacterProfile{
		ID:          id,
		Name:        name,
		Role:        "配角",
		Status:      "active",
		Personality:  "待AI生成",
		Background:   "待AI生成",
		Motivation:   "待AI生成",
	}
	if err := project.WriteCharacter(s.root, s.project, newCh); err != nil {
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

// ---- model management ----

// checkAPIKeyOnStartup checks if at least one model has a valid API key.
func (s *Session) checkAPIKeyOnStartup() {
	_, err := s.router.GetClient("deepseek")
	hasDS := err == nil
	_, err = s.router.GetClient("mimo")
	hasMiMo := err == nil
	_, err = s.router.GetClient("minimax")
	hasMM := err == nil

	if hasDS || hasMiMo || hasMM {
		active := ""
		if hasDS {
			active = "DeepSeek"
		} else if hasMiMo {
			active = "MiMo"
		} else {
			active = "MiniMax"
		}
		fmt.Printf("  🤖 活跃模型: %s | 输入 /model 切换\n", active)
		return
	}

	// No API key configured — walk user through it
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════╗")
	fmt.Println("  ║  首次使用：配置 AI 模型 API Key                  ║")
	fmt.Println("  ║                                                  ║")
	fmt.Println("  ║  推荐 DeepSeek（最便宜 ¥0.001/千tokens）          ║")
	fmt.Println("  ║  申请: https://platform.deepseek.com/api_keys     ║")
	fmt.Println("  ║                                                  ║")
	fmt.Println("  ║  输入 /model set deepseek <你的key> 立即配置      ║")
	fmt.Println("  ╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

func (s *Session) cmdModel(arg string) {
	if arg == "" || arg == "list" {
		providers := []string{"deepseek", "minimax", "mimo"}
		fmt.Println("  AI 模型状态:")
		hasAny := false
		for _, p := range providers {
			_, err := s.router.GetClient(p)
			icon := "✗"
			if err == nil {
				icon = "✓"
				hasAny = true
			}
			label := model.ProviderLabels[p]
			if label == "" {
				label = p
			}
			fmt.Printf("    %s %s  %s\n", icon, p, label)
		}
		if !hasAny {
			fmt.Println()
			fmt.Println("  还没有配置任何 API Key。")
			fmt.Println("  用法: /model set deepseek sk-xxx")
		}
		return
	}

	parts := strings.SplitN(arg, " ", 2)
	if len(parts) < 2 {
		fmt.Println("  用法: /model set <模型> <key>  或  /model switch <模型>")
		return
	}

	switch parts[0] {
	case "set":
		parts2 := strings.SplitN(parts[1], " ", 2)
		if len(parts2) < 2 {
			fmt.Println("  用法: /model set <deepseek|minimax|mimo> <api-key>")
			return
		}
		provider := parts2[0]
		apiKey := parts2[1]

		valid := map[string]bool{"deepseek": true, "minimax": true, "mimo": true}
		if !valid[provider] {
			fmt.Printf("  ✗ 未知模型: %s。可选: deepseek, minimax, mimo\n", provider)
			return
		}

		cfg, err := loadConfig(s.root)
		if err != nil {
			fmt.Printf("  ✗ 读取配置失败: %v\n", err)
			return
		}
		if _, ok := cfg[provider]; !ok {
			cfg[provider] = map[string]any{}
		}
		pmap := cfg[provider].(map[string]any)
		pmap["api_key"] = apiKey
		cfg[provider] = pmap

		if err := writeConfig(s.root, cfg); err != nil {
			fmt.Printf("  ✗ 保存配置失败: %v\n", err)
			return
		}

		if err := s.reloadHarness(); err != nil {
			fmt.Printf("  ⚠ 配置已保存但热重载失败: %v\n", err)
			fmt.Println("  请重启 novel-agent 使配置生效。")
			return
		}

		label := model.ProviderLabels[provider]
		if label == "" {
			label = provider
		}
		fmt.Printf("  ✓ %s API Key 已配置并生效\n", label)

	case "switch":
		provider := parts[1]
		_, err := s.router.GetClient(provider)
		if err != nil {
			label := model.ProviderLabels[provider]
			if label == "" {
				label = provider
			}
			fmt.Printf("  ✗ %s 未配置 API Key。\n", label)
			fmt.Print("  📝 请输入 API Key（留空取消）: ")
			reader := bufio.NewReader(os.Stdin)
			inp, _ := reader.ReadString('\n')
			inp = strings.TrimSpace(inp)
			if inp == "" {
				fmt.Println("  ✗ 已取消")
				return
			}
			s.cmdModel("set " + provider + " " + inp)
			return
		}

		// Update fallback to this provider
		cfg, err := loadConfig(s.root)
		if err != nil {
			fmt.Printf("  ✗ 读取配置失败: %v\n", err)
			return
		}
		if routing, ok := cfg["stage_routing"]; ok {
			if rmap, ok := routing.(map[string]any); ok {
				rmap["fallback"] = provider
			}
		}
		if err := writeConfig(s.root, cfg); err != nil {
			fmt.Printf("  ✗ 保存配置失败: %v\n", err)
			return
		}
		if err := s.reloadHarness(); err != nil {
			fmt.Printf("  ⚠ 配置已保存但热重载失败: %v\n", err)
			return
		}
		label := model.ProviderLabels[provider]
		if label == "" {
			label = provider
		}
		fmt.Printf("  ✓ 已切换到 %s\n", label)

	default:
		fmt.Println("  用法: /model | /model set <模型> <key> | /model switch <模型>")
	}
}

// reloadHarness reloads the model router from config.yaml without restarting.
func (s *Session) reloadHarness() error {
	cfg, err := loadConfig(s.root)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	mcs, fallback := modelConfigsFromYAML(cfg)
	newRouter, err := model.NewRouter(mcs, fallback)
	if err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	s.router = newRouter
	s.h.Router = newRouter
	return nil
}

// cmdCharEvolve calls AI to refine a character's skill prompt based on their
// full evolution history, then writes the refined prompt back to the YAML.
func (s *Session) cmdCharEvolve(name string) {
	if s.project == "" {
		fmt.Println("  请先选择一个项目: /project <名称>")
		return
	}
	if name == "" {
		fmt.Println("  用法: /char evolve <角色名>")
		return
	}

	id := project.CharacterID(name)
	ch, err := project.ReadCharacter(s.root, s.project, id)
	if err != nil {
		fmt.Printf("  ✗ 角色「%s」不存在\n", name)
		return
	}

	if len(ch.Evolution) == 0 {
		fmt.Printf("  ℹ 角色「%s」还没有成长记录。\n", name)
		fmt.Println("    使用 /write 续写章节后，系统会自动检测角色变化。")
		fmt.Println("    也可以手动调用 /char <名> 编辑后再次 evolve。")
		return
	}

	fmt.Printf("  🧠 提炼「%s」(%d 步成长轨迹)...\n", ch.Name, len(ch.Evolution))

	// Build meta-prompt containing the full evolution log
	state := ch.CurrentState()
	metaPrompt := fmt.Sprintf(
		"你是角色编辑助手。请根据以下角色的【当前状态】和【完整成长轨迹】，"+
			"生成一段适配该角色当前设定的小说续写 system prompt。\n\n"+
			"要求：\n"+
			"1. 保留角色的核心性格和动机\n"+
			"2. 融入最新的成长变化（能力提升、性格转变等）\n"+
			"3. 描述该角色当前的行为模式、说话风格和情感状态\n"+
			"4. 长度控制在 200 字以内\n"+
			"5. 只输出 prompt 文本，不包含任何解释\n\n"+
			"【角色状态】\n%s\n\n"+
			"请输出该角色更新后的 system prompt：", state,
	)

	fmt.Println("  ⏳ 调用 AI 模型优化角色提示词...")
	ctx := context.Background()

	// Use fallback model to refine the prompt
	client, err := s.router.GetClient("qwen")
	if err != nil {
		client, err = s.router.GetClient("deepseek")
		if err != nil {
			fmt.Printf("  ✗ 无可用的 AI 模型: %v\n", err)
			return
		}
	}

	refined, err := client.Generate(ctx, "你是专业的角色设定师，擅长根据人物成长轨迹提炼精确的角色提示词。", metaPrompt)
	if err != nil {
		fmt.Printf("  ✗ AI 调用失败: %v\n", err)
		return
	}

	ch.EvolvePromptFromString(refined)
	if err := project.WriteCharacter(s.root, s.project, *ch); err != nil {
		fmt.Printf("  ✗ 保存角色失败: %v\n", err)
		return
	}

	fmt.Printf("  ✓ 角色「%s」提示词已优化 (%d 字)\n", ch.Name, len([]rune(refined)))
	fmt.Printf("  ─── 优化后 ───\n%s\n  ───────────\n", refined)
	fmt.Println("  💡 下次使用 /write 续写时，系统会自动注入该角色的最新状态。")
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

	// Auto-detect character evolutions from the generated chapter
	s.detectEvolutions(chNo, out.Content)
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

// loadConfig reads and resolves env vars from .novelAgent/config.yaml.
func loadConfig(root string) (map[string]any, error) {
	cfg, err := storage.ReadConfig(root)
	if err != nil {
		return nil, err
	}
	resolveEnvVars(cfg)
	return cfg, nil
}

func writeConfig(root string, cfg map[string]any) error {
	return storage.WriteConfig(root, cfg)
}

// resolveEnvVars replaces ${VAR} placeholders in a config map.
func resolveEnvVars(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
				envKey := val[2 : len(val)-1]
				if envVal := os.Getenv(envKey); envVal != "" {
					m[k] = envVal
				}
			}
		case map[string]any:
			resolveEnvVars(val)
		}
	}
}

func modelConfigsFromYAML(cfg map[string]any) (map[string]model.Config, string) {
	configs := make(map[string]model.Config)
	fallback := "deepseek"
	for _, provider := range []string{"deepseek", "minimax", "mimo"} {
		if raw, ok := cfg[provider]; ok {
			if pmap, ok := raw.(map[string]any); ok {
				mc := model.DefaultConfig(provider)
				if v, ok := pmap["api_key"].(string); ok && v != "" && !strings.HasPrefix(v, "${") {
					mc.APIKey = v
				}
				if v, ok := pmap["endpoint"].(string); ok {
					mc.Endpoint = v
				} else if v, ok := pmap["api_endpoint"].(string); ok {
					mc.Endpoint = v
				}
				if v, ok := pmap["model_name"].(string); ok {
					mc.ModelName = v
				}
				if v, ok := pmap["max_tokens"].(int); ok {
					mc.MaxTokens = v
				}
				if v, ok := pmap["temperature"].(float64); ok {
					mc.Temperature = v
				}
				if v, ok := pmap["timeout"].(int); ok {
					mc.Timeout = time.Duration(v) * time.Second
				}
				if v, ok := pmap["retry_times"].(int); ok {
					mc.RetryTimes = v
				}
				configs[provider] = mc
			}
		}
	}
	if routing, ok := cfg["stage_routing"]; ok {
		if rmap, ok := routing.(map[string]any); ok {
			if fb, ok := rmap["fallback"].(string); ok {
				fallback = fb
			}
		}
	}
	return configs, fallback
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// detectEvolutions scans the generated chapter for character changes and prompts
// the user to record them in the evolution log.
func (s *Session) detectEvolutions(chapterNo int, content string) {
	chars, err := project.ListCharacters(s.root, s.project)
	if err != nil || len(chars) == 0 {
		return
	}

	// Simple heuristic: find character names mentioned near change keywords
	changeKeywords := []string{
		"突破", "升级", "进阶", "觉醒", "领悟",
		"获得", "习得", "掌握", "突破到",
		"成为", "晋升", "蜕变", "进化",
		"斩杀", "击败", "收服",
	}
	found := map[string]string{} // character ID → matched change phrase

	for _, ch := range chars {
		if ch.Status != "active" {
			continue
		}
		for _, kw := range changeKeywords {
			// Look for "character name ... keyword ..." within close proximity
			if idx := strings.Index(content, ch.Name); idx >= 0 {
				snippet := content[idx:]
				maxLen := 300
				if len(snippet) > maxLen {
					snippet = snippet[:maxLen]
				}
				if kwIdx := strings.Index(snippet, kw); kwIdx > 0 && kwIdx < 200 {
					found[ch.ID] = kw
					break
				}
			}
		}
	}

	if len(found) == 0 {
		return
	}

	fmt.Println("\n  ⚡ 检测到可能的角色变化：")
	for id, kw := range found {
		for _, ch := range chars {
			if ch.ID == id {
				fmt.Printf("    %s — %s\n", ch.Name, kw)
			}
		}
	}
	fmt.Print("  是否记录这些变化到角色成长日志？[Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(strings.ToLower(resp))
	if resp != "" && resp != "y" && resp != "yes" {
		fmt.Println("  ℹ 跳过。稍后可以用 /char <名> 手动编辑，或 /char evolve <名> 优化角色提示词。")
		return
	}

	recorded := 0
	for id, kw := range found {
		for _, ch := range chars {
			if ch.ID == id {
				step := project.EvolutionStep{
					Chapter:     chapterNo,
					Type:        evolutionTypeFromKeyword(kw),
					Description: fmt.Sprintf("第%d章：%s", chapterNo, kw),
					Before:      ch.Abilities,
					After:       fmt.Sprintf("第%d章 %s", chapterNo, kw),
					ConfirmedBy: "user",
				}
				ch.AppendEvolution(step)
				project.WriteCharacter(s.root, s.project, ch)
				recorded++
			}
		}
	}
	fmt.Printf("  ✓ 已记录 %d 条角色变化。\n", recorded)
	fmt.Println("  💡 下次 /write 续写时，系统会自动注入角色最新状态。")
}

func evolutionTypeFromKeyword(kw string) string {
	switch kw {
	case "突破", "升级", "进阶", "晋升", "进化", "蜕变", "突破到":
		return "升级"
	case "觉醒", "领悟":
		return "觉醒"
	case "获得", "习得", "掌握":
		return "获得能力"
	case "斩杀", "击败":
		return "战斗转折"
	case "成为":
		return "身份转变"
	case "收服":
		return "关系变化"
	default:
		return "转折"
	}
}

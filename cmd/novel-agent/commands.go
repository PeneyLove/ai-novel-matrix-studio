package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/harness"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/model"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/pipeline"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/skill"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/storage"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/api"
	"github.com/PeneyLove/ai-novel-matrix-studio/skilldata"
)

// --- Helpers ---

func agentRoot() string {
	// Use .novelAgent in the current working directory
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, storage.Dir)
}

func loadConfig(root string) (map[string]any, error) {
	cfg, err := storage.ReadConfig(root)
	if err != nil {
		return nil, err
	}
	// Resolve ${ENV_VAR} placeholders
	resolveEnvVars(cfg)
	return cfg, nil
}

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
	fallback := "qwen"

	// Parse model configs
	for _, provider := range []string{"minimax", "doubao", "qwen", "deepseek"} {
		if raw, ok := cfg[provider]; ok {
			if pmap, ok := raw.(map[string]any); ok {
				mc := model.DefaultConfig(provider)
				if v, ok := pmap["api_key"].(string); ok {
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

	// Parse stage_routing for fallback
	if routing, ok := cfg["stage_routing"]; ok {
		if rmap, ok := routing.(map[string]any); ok {
			if fb, ok := rmap["fallback"].(string); ok {
				fallback = fb
			}
		}
	}

	return configs, fallback
}

func newHarness() (*harness.Harness, error) {
	root := agentRoot()
	cfg, err := loadConfig(root)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w — run 'novel-agent init' first", err)
	}
	mcs, fallback := modelConfigsFromYAML(cfg)
	return harness.New(root, mcs, fallback)
}

// --- init ---

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .novelAgent/ directory",
		Long:  "Creates the standard .novelAgent/ directory structure with default config and built-in skill definitions.",
		RunE:  runInit,
	}
	cmd.Flags().Bool("force", false, "Overwrite existing config.yaml")
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	root := agentRoot()
	force, _ := cmd.Flags().GetBool("force")

	// Create directory structure
	dirs := []string{
		filepath.Join(root, "skills"),
		filepath.Join(root, "corpus"),
		filepath.Join(root, "outputs"),
		filepath.Join(root, "traces"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}

	// Write default config if not exists
	configPath := filepath.Join(root, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) || force {
		defaultConfig := map[string]any{
			"minimax": map[string]any{
				"api_key":      "${MINIMAX_API_KEY}",
				"api_endpoint": "https://api.minimax.chat/v1/text/chatcompletion_v2",
				"model_name":   "abab6.5s-chat",
				"max_tokens":   4096,
				"temperature":  0.8,
				"timeout":      60,
				"retry_times":  3,
			},
			"doubao": map[string]any{
				"api_key":      "${DOUBAO_API_KEY}",
				"api_endpoint": "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
				"model_name":   "doubao-pro-32k",
				"max_tokens":   8192,
				"temperature":  0.7,
				"timeout":      90,
				"retry_times":  3,
			},
			"qwen": map[string]any{
				"api_key":      "${QWEN_API_KEY}",
				"api_endpoint": "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation",
				"model_name":   "qwen-long",
				"max_tokens":   6000,
				"temperature":  0.75,
				"timeout":      120,
				"retry_times":  3,
			},
			"deepseek": map[string]any{
				"api_key":      "${DEEPSEEK_API_KEY}",
				"api_endpoint": "https://api.deepseek.com/v1/chat/completions",
				"model_name":   "deepseek-chat",
				"max_tokens":   4096,
				"temperature":  0.6,
				"timeout":      60,
				"retry_times":  3,
			},
			"stage_routing": map[string]any{
				"topic_generation":   "minimax",
				"outline_generation": "doubao",
				"content_generation": "qwen",
				"polish":             "deepseek",
				"fallback":           "qwen",
			},
			"global_rules": map[string]any{
				"language": "zh-CN",
				"rules": []string{
					"全程使用简体中文输出，包括所有说明、描述、对话、叙述",
					"专有名词、技术术语可保留原文（如 API、SDK、GDP、CEO 等）",
					"人名、地名、品牌名等专有名称可保留英文或拼音原文",
					"网络热梗、流行语、meme 可以使用，但涉及实时信息时需申请联网权限",
					"代码块、命令行示例保持英文原样",
					"禁止输出繁体中文",
					"数字使用阿拉伯数字",
					"标点符号使用全角中文标点",
				},
				"network": map[string]any{
					"enabled":        false,
					"ask_permission": true,
				},
			},
		}
		if err := storage.WriteConfig(root, defaultConfig); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		cmd.Println("✓ Created config.yaml (edit to add your API keys)")
	} else {
		cmd.Println("ⓘ config.yaml already exists (use --force to overwrite)")
	}

	// Copy built-in skills (v2.0 genre system) from embedded FS
	builtinGenres := []string{"xuanhuan", "dushi", "guyan", "xuanyi", "kehuan", "tianchong"}
	builtinSubSkills := []string{"genre_init", "outline", "hooks", "writing"}
	builtinOptimizes := []string{"shuangdian", "fubi", "jiezou", "renshe", "chongtu"}
	installed := 0
	for _, genre := range builtinGenres {
		for _, sub := range builtinSubSkills {
			path := filepath.Join("skills", genre, sub+".yaml")
			if err := installSkillFromEmbed(root, path, genre+"_"+sub); err == nil {
				installed++
			}
		}
		for _, opt := range builtinOptimizes {
			path := filepath.Join("skills", genre, "optimize", opt+".yaml")
			if err := installSkillFromEmbed(root, path, genre+"_optimize_"+opt); err == nil {
				installed++
			}
		}
	}

	// Validate
	mgr, err := skill.NewManager(root)
	if err != nil {
		return fmt.Errorf("validate skills: %w", err)
	}
	names := mgr.List()
	if len(names) == 0 {
		cmd.Println("⚠ No skills loaded — install skills with 'novel-agent skill install <name>'")
	} else {
		cmd.Printf("✓ Initialized with %d skill(s): %s\n", len(names), strings.Join(names, ", "))
	}

	cmd.Println("\nNext steps:")
	cmd.Println("  1. Edit .novelAgent/config.yaml to add your API keys")
	cmd.Println("  2. Run: novel-agent skill list")
	cmd.Println("  3. Run: novel-agent pipeline --skill xuanhuan_genre_init --trend-data \"你的选题方向\"")
	return nil
}

func installSkillFromEmbed(root, embedPath, skillName string) error {
	data, err := skilldata.FS.ReadFile(embedPath)
	if err != nil {
		return err
	}
	var skillMap map[string]any
	if err := yaml.Unmarshal(data, &skillMap); err != nil {
		return err
	}
	return storage.WriteSkill(root, skillName, skillMap)
}

// --- run ---

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run --skill <name> --stage <stage> --input <json-string>",
		Short: "Execute a single creation stage",
		Example: `  novel-agent run --skill female_rebirth --stage topic_generation \
    --input '{"trend_data":"重生虐渣文持续霸榜"}'`,
		RunE: runRun,
	}
	cmd.Flags().String("skill", "", "Skill name (required)")
	cmd.Flags().String("stage", "", "Stage: topic_generation|outline_generation|content_generation|polish (required)")
	cmd.Flags().String("input", "", "Input data as JSON string or @filepath")
	cmd.Flags().String("task-id", "", "Task ID (auto-generated if empty)")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	skillName, _ := cmd.Flags().GetString("skill")
	stage, _ := cmd.Flags().GetString("stage")
	inputStr, _ := cmd.Flags().GetString("input")
	taskID, _ := cmd.Flags().GetString("task-id")

	if skillName == "" || stage == "" || inputStr == "" {
		return fmt.Errorf("--skill, --stage, and --input are required")
	}

	// Parse input
	if strings.HasPrefix(inputStr, "@") {
		data, err := os.ReadFile(inputStr[1:])
		if err != nil {
			return fmt.Errorf("read input file: %w", err)
		}
		inputStr = string(data)
	}
	var inputMap map[string]string
	if err := json.Unmarshal([]byte(inputStr), &inputMap); err != nil {
		return fmt.Errorf("parse input JSON: %w", err)
	}

	if taskID == "" {
		taskID = fmt.Sprintf("task-%d", time.Now().Unix())
	}

	h, err := newHarness()
	if err != nil {
		return err
	}
	defer h.Close()

	input := pipeline.StageInput{
		TrendData:      inputMap["trend_data"],
		Topic:          inputMap["topic"],
		ChapterOutline: inputMap["chapter_outline"],
		PrevContext:    inputMap["prev_context"],
		Content:        inputMap["content"],
	}

	ctx := context.Background()
	out, err := h.RunStage(ctx, taskID, skillName, stage, input)
	if err != nil {
		return err
	}

	cmd.Printf("✓ Stage %q completed for task %q\n", out.Stage, out.TaskID)
	cmd.Printf("  Content length: %d chars\n", len(out.Content))
	cmd.Printf("  Prompt hash: %s\n", out.PromptHash)
	cmd.Printf("  Draft hash:  %s\n", out.DraftHash)
	cmd.Printf("\n--- BEGIN OUTPUT ---\n%s\n--- END OUTPUT ---\n", out.Content)

	return nil
}

// --- pipeline ---

func pipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline --skill <name> --trend-data <text>",
		Short: "Run the full creation pipeline (topic→outline→content→polish)",
		Example: `  novel-agent pipeline --skill female_rebirth \
    --trend-data "近期热榜：重生、穿越、虐渣"`,
		RunE: runPipeline,
	}
	cmd.Flags().String("skill", "", "Skill name (required)")
	cmd.Flags().String("trend-data", "", "Trend data for topic generation (required)")
	cmd.Flags().String("task-id", "", "Task ID (auto-generated if empty)")
	return cmd
}

func runPipeline(cmd *cobra.Command, args []string) error {
	skillName, _ := cmd.Flags().GetString("skill")
	trendData, _ := cmd.Flags().GetString("trend-data")
	taskID, _ := cmd.Flags().GetString("task-id")

	if skillName == "" || trendData == "" {
		return fmt.Errorf("--skill and --trend-data are required")
	}
	if taskID == "" {
		taskID = fmt.Sprintf("task-%d", time.Now().Unix())
	}

	h, err := newHarness()
	if err != nil {
		return err
	}
	defer h.Close()

	ctx := context.Background()
	outputs, err := h.RunPipeline(ctx, taskID, skillName, trendData)
	if err != nil {
		return err
	}

	cmd.Printf("✓ Pipeline completed for task %q (%d stages)\n\n", taskID, len(outputs))
	for _, out := range outputs {
		preview := out.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		cmd.Printf("  [%s] %d chars | hash=%s\n", out.Stage, len(out.Content), out.DraftHash[:16])
	}
	cmd.Printf("\nFull outputs saved in .novelAgent/outputs/%s/\n", taskID)
	return nil
}

// --- skill ---

func skillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage skills (list, install, validate, remove)",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed skills",
			RunE: func(cmd *cobra.Command, args []string) error {
				h, err := newHarness()
				if err != nil {
					return err
				}
				names := h.ListSkills()
				if len(names) == 0 {
					cmd.Println("No skills installed.")
					return nil
				}
				for _, name := range names {
					sk := h.GetSkill(name)
					if sk != nil {
						cmd.Printf("  %-20s v%-8s %s\n", sk.Name, sk.Version, sk.Description)
					} else {
						cmd.Printf("  %s\n", name)
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "validate <skill-name>",
			Short: "Validate a skill YAML definition",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				root := agentRoot()
				l := skill.NewLoader(root)
				sk, err := l.Load(args[0])
				if err != nil {
					return err
				}
				cmd.Printf("✓ Skill %q v%s is valid\n", sk.Name, sk.Version)
				cmd.Printf("  Stages: %s\n", strings.Join(sk.Stages, ", "))
				return nil
			},
		},
		&cobra.Command{
			Use:   "remove <skill-name>",
			Short: "Remove an installed skill",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				root := agentRoot()
				skillDir := filepath.Join(root, "skills", args[0])
				if err := os.RemoveAll(skillDir); err != nil {
					return fmt.Errorf("remove skill %q: %w", args[0], err)
				}
				cmd.Printf("✓ Removed skill %q\n", args[0])
				return nil
			},
		},
	)
	return cmd
}

// --- export ---

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export --task-id <id> --format txt",
		Short: "Export generated content",
		RunE:  runExport,
	}
	cmd.Flags().String("task-id", "", "Task ID (required)")
	cmd.Flags().String("format", "txt", "Output format: txt")
	cmd.Flags().String("output", "", "Output file path (default: ./export-<task-id>.txt)")
	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	taskID, _ := cmd.Flags().GetString("task-id")
	format, _ := cmd.Flags().GetString("format")
	outputPath, _ := cmd.Flags().GetString("output")

	if taskID == "" {
		return fmt.Errorf("--task-id is required")
	}
	if outputPath == "" {
		outputPath = fmt.Sprintf("export-%s.%s", taskID[:min(8, len(taskID))], format)
	}

	root := agentRoot()

	// Read all stage outputs
	entries, err := os.ReadDir(filepath.Join(root, "outputs", taskID))
	if err != nil {
		return fmt.Errorf("task %q not found: %w", taskID, err)
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("AI Novel Agent — Export\n"))
	buf.WriteString(fmt.Sprintf("Task ID: %s\n", taskID))
	buf.WriteString(fmt.Sprintf("Export time: %s\n", time.Now().Format(time.RFC3339)))
	buf.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		content, err := storage.ReadOutput(root, taskID, e.Name())
		if err != nil {
			continue
		}
		stage := strings.TrimSuffix(e.Name(), ".txt")
		buf.WriteString(fmt.Sprintf("--- %s ---\n\n", stage))
		buf.WriteString(content)
		buf.WriteString("\n\n")
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}

	info, _ := os.Stat(outputPath)
	cmd.Printf("✓ Exported to %s (%d bytes)\n", outputPath, info.Size())
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startAPIServer boots the HTTP API for the Flutter GUI.
func startAPIServer(h *harness.Harness, port int) error {
	server := api.NewServer(h, port)
	return server.Run()
}

// --- serve ---

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start local HTTP API server (for Flutter GUI)",
		Long:  "Starts a lightweight HTTP server on 127.0.0.1:9876 for the Flutter GUI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")

			h, err := newHarness()
			if err != nil {
				return err
			}
			defer h.Close()

			// Import api package dynamically
			// We use a simple inline pattern to avoid circular imports
			server := startAPIServer(h, port)
			return server
		},
	}
	cmd.Flags().Int("port", 9876, "HTTP listen port")
	return cmd
}

// --- migrate ---

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data (not needed — v1.x starts fresh)",
		Long:  "v1.x stores all data locally in .novelAgent/. No migration from v0.x is required. Start fresh with 'novel-agent init'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("v1.x uses local .novelAgent/ storage — no external database migration needed.")
			cmd.Println("Run 'novel-agent init' to start fresh.")
			return nil
		},
	}
	return cmd
}

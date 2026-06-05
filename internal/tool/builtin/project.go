package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

type projectSwitchTool struct{}

func init() { tool.RegisterBuiltin(projectSwitchTool{}) }

func (t projectSwitchTool) Name() string        { return "project_switch" }
func (t projectSwitchTool) ReadOnly() bool       { return false }
func (t projectSwitchTool) Description() string  { return "切换到指定项目（不存在则创建）；列出所有项目" }
func (t projectSwitchTool) Schema() json.RawMessage {
	return tool.ObjSchema(tool.Prop("name", "string", "项目名称（留空列出所有项目）"), nil)
}

func (t projectSwitchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Name string `json:"name"`
	}
	json.Unmarshal(args, &p)

	root := toolRoot(ctx)
	if p.Name == "" {
		projs, _ := project.ListProjects(root)
		if len(projs) == 0 {
			return "暂无项目", nil
		}
		data, _ := json.Marshal(projs)
		return string(data), nil
	}

	dir := project.Dir(root, p.Name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := project.Init(root, p.Name); err != nil {
			return "", fmt.Errorf("创建项目失败: %w", err)
		}
	}
	return fmt.Sprintf(`{"switched":"%s"}`, p.Name), nil
}

// toolRoot reads the project root from context.
func toolRoot(ctx context.Context) string {
	r := tool.RootFrom(ctx)
	if r == "" {
		r = ".novelAgent"
	}
	return r
}

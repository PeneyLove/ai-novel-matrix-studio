package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

type writeTool struct{}

func init() { tool.RegisterBuiltin(writeTool{}) }

func (t writeTool) Name() string       { return "chapter_write" }
func (t writeTool) ReadOnly() bool     { return false }
func (t writeTool) Description() string { return "调用 AI 续写指定章节，结果保存到 output/；返回章节预览" }

func (t writeTool) Schema() json.RawMessage {
	return tool.ObjSchema(tool.MergeProps(
		tool.Prop("project", "string", "项目名称"),
		tool.Prop("chapter", "integer", "章节号（1-based）"),
		tool.Prop("outline_hint", "string", "本章大纲提示（可选）"),
	), []string{"project", "chapter"})
}

func (t writeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Project     string `json:"project"`
		Chapter     int    `json:"chapter"`
		OutlineHint string `json:"outline_hint"`
	}
	json.Unmarshal(args, &p)
	if p.Chapter <= 0 {
		p.Chapter = 1
	}
	return fmt.Sprintf(`{"status":"delegated","chapter":%d,"note":"请在 TUI 输入 /write %d 执行续写","outline_hint":"%s"}`, p.Chapter, p.Chapter, p.OutlineHint), nil
}

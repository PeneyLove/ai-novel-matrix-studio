package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

type worldTool struct{}

func init() { tool.RegisterBuiltin(worldTool{}) }

func (t worldTool) Name() string        { return "world" }
func (t worldTool) ReadOnly() bool      { return false }
func (t worldTool) Description() string { return "查看或修改世界观设定（genre/sub_genre/power_system/factions）" }

func (t worldTool) Schema() json.RawMessage {
	return tool.ObjSchema(tool.MergeProps(
		tool.Prop("project", "string", "项目名称（可选）"),
		tool.Prop("action", "string", "view|set（view 查看，set 修改字段）"),
		tool.Prop("genre", "string", "小说类型（xuanhuan/dushi/guyan/xuanyi/kehuan/tianchong）"),
		tool.Prop("sub_genre", "string", "细分流派"),
		tool.Prop("power_system", "string", "力量体系描述"),
		tool.Prop("factions", "string", "势力列表，逗号分隔"),
	), []string{"action"})
}

func (t worldTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Project     string `json:"project"`
		Action      string `json:"action"`
		Genre       string `json:"genre"`
		SubGenre    string `json:"sub_genre"`
		PowerSystem string `json:"power_system"`
		Factions    string `json:"factions"`
	}
	json.Unmarshal(args, &p)
	root := toolRoot(ctx)

	w, err := project.ReadWorld(root, p.Project)
	if err != nil {
		return tool.Err("读取世界设定失败: " + err.Error()), nil
	}

	switch p.Action {
	case "view":
		b, _ := json.Marshal(w)
		return string(b), nil
	case "set":
		if p.Genre != "" {
			w["genre"] = p.Genre
		}
		if p.SubGenre != "" {
			w["sub_genre"] = p.SubGenre
		}
		if p.PowerSystem != "" {
			w["power_system"] = p.PowerSystem
		}
		if p.Factions != "" {
			factions := []any{}
			for _, f := range strings.Split(p.Factions, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					factions = append(factions, f)
				}
			}
			w["factions"] = factions
		}
		if err := project.WriteWorld(root, p.Project, w); err != nil {
			return tool.Err("保存失败: " + err.Error()), nil
		}
		return fmt.Sprintf(`{"updated":true,"genre":"%s","power_system":"%s"}`, w["genre"], w["power_system"]), nil
	default:
		return tool.Err("用法: /world view 或 /world set genre=xxx"), nil
	}
}

package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/project"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

type charTool struct{}

func init() { tool.RegisterBuiltin(charTool{}) }

func (t charTool) Name() string       { return "character" }
func (t charTool) ReadOnly() bool     { return false }
func (t charTool) Description() string { return "创建/查看/下线角色；action: create|view|list|deactivate|evolve" }

func (t charTool) Schema() json.RawMessage {
	return tool.ObjSchema(tool.MergeProps(
		tool.Prop("action", "string", "create|view|list|deactivate|evolve"),
		tool.Prop("project", "string", "项目名称（可选，默认当前项目）"),
		tool.Prop("name", "string", "角色名（create/view/deactivate 时必填）"),
		tool.Prop("role", "string", "角色定位：主角|配角|反派|路人（create 时可选，默认配角）"),
		tool.Prop("personality", "string", "性格描述（create/evolve 时可选）"),
		tool.Prop("reason", "string", "下线原因（deactivate 时必填）"),
	), []string{"action"})
}

func (t charTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Action      string `json:"action"`
		Project     string `json:"project"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		Personality string `json:"personality"`
		Reason      string `json:"reason"`
	}
	json.Unmarshal(args, &p)
	root := toolRoot(ctx)

	switch p.Action {
	case "list":
		chars, err := project.ListCharacters(root, p.Project)
		if err != nil {
			return tool.Err(err.Error()), nil
		}
		if len(chars) == 0 {
			return "[]", nil
		}
		type charSummary struct {
			Name   string `json:"name"`
			Role   string `json:"role"`
			Status string `json:"status"`
		}
		out := make([]charSummary, 0, len(chars))
		for _, ch := range chars {
			out = append(out, charSummary{ch.Name, ch.Role, ch.Status})
		}
		b, _ := json.Marshal(out)
		return string(b), nil

	case "create":
		if p.Name == "" {
			return tool.Err("角色名不能为空"), nil
		}
		if p.Role == "" {
			p.Role = "配角"
		}
		id := project.CharacterID(p.Name)
		ch := project.CharacterProfile{
			ID: id, Name: p.Name, Role: p.Role, Status: "active",
			Personality: p.Personality, Background: "待设定", Motivation: "待设定",
		}
		if err := project.WriteCharacter(root, p.Project, ch); err != nil {
			return tool.Err(err.Error()), nil
		}
		return fmt.Sprintf(`{"created":"%s","id":"%s","role":"%s"}`, p.Name, id, p.Role), nil

	case "view":
		id := project.CharacterID(p.Name)
		ch, err := project.ReadCharacter(root, p.Project, id)
		if err != nil {
			return tool.Err("角色不存在: " + p.Name), nil
		}
		b, _ := json.Marshal(ch)
		return string(b), nil

	case "deactivate":
		id := project.CharacterID(p.Name)
		summary, err := project.DeactivateCharacter(root, p.Project, id, p.Reason)
		if err != nil {
			return tool.Err(err.Error()), nil
		}
		return fmt.Sprintf(`{"deactivated":"%s","summary":"%s"}`, p.Name, summary), nil

	case "evolve":
		id := project.CharacterID(p.Name)
		ch, err := project.ReadCharacter(root, p.Project, id)
		if err != nil {
			return tool.Err("角色不存在"), nil
		}
		return ch.CurrentState(), nil

	default:
		return tool.Err("未知 action: " + p.Action + "。可选: create|view|list|deactivate|evolve"), nil
	}
}

package global_test

import (
	"strings"
	"testing"

	"github.com/penney-101/ai-novel-agent/internal/global"
)

func TestDefaultRulesAsPromptPrefix(t *testing.T) {
	rules := global.DefaultRules()
	prefix := rules.AsPromptPrefix()

	// Must contain the header
	if !strings.Contains(prefix, "【全局规则 — 必须遵守】") {
		t.Error("prefix missing header")
	}

	// Must contain each of the 8 default rules
	requiredSnippets := []string{
		"简体中文",
		"专有名词",
		"保留英文",
		"网络热梗",
		"代码块",
		"繁体中文",
		"阿拉伯数字",
		"全角中文标点",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(prefix, snippet) {
			t.Errorf("prefix missing rule containing %q", snippet)
		}
	}

	// Must have numbered lines
	if !strings.Contains(prefix, "1. ") || !strings.Contains(prefix, "8. ") {
		t.Error("rules should be numbered 1-8")
	}
}

func TestDefaultRulesLanguageIsZhCN(t *testing.T) {
	rules := global.DefaultRules()
	if rules.Language != "zh-CN" {
		t.Errorf("Language = %q, want zh-CN", rules.Language)
	}
}

func TestDefaultRulesNetworkDisabled(t *testing.T) {
	rules := global.DefaultRules()
	if rules.Network.Enabled {
		t.Error("Network.Enabled should be false by default")
	}
	if !rules.Network.AskPermission {
		t.Error("Network.AskPermission should be true by default")
	}
}

func TestCheckPermissionWhenDisabled(t *testing.T) {
	rules := global.DefaultRules()
	permReq := global.CheckPermission(rules, false, "xuanhuan_hot_meme", "需要搜索网络热梗")
	if permReq == nil {
		t.Error("expected permission request when network is disabled and AskPermission=true")
	}
	if !strings.Contains(permReq.String(), "联网权限") {
		t.Errorf("permission request string should mention 联网权限: %s", permReq.String())
	}
}

func TestCheckPermissionWhenEnabled(t *testing.T) {
	rules := global.DefaultRules()
	rules.Network.Enabled = true
	permReq := global.CheckPermission(rules, true, "xuanhuan_hot_meme", "需要搜索网络热梗")
	if permReq != nil {
		t.Error("expected nil when network is enabled")
	}
}

func TestRulesFieldCount(t *testing.T) {
	rules := global.DefaultRules()
	if len(rules.Rules) != 8 {
		t.Errorf("expected 8 default rules, got %d", len(rules.Rules))
	}
}

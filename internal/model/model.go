// Package model provides AI model clients with deep compatibility for domestic LLMs.
//
// Supported providers and their idiosyncrasies:
//
//	DeepSeek — OpenAI-compatible, supports frequency_penalty, stop tokens
//	MiniMax  — requires group_id, reply_constraints, sender_type messages
//	MiMo     — OpenAI-compatible (Xiaomi), token-plan endpoint
//
// Each client handles its own request/response serialization natively
// to avoid the "lowest common denominator" problem.
package model

import (
	"context"
	"fmt"
	"time"
)

// Provider identifiers.
const (
	ProviderDeepSeek = "deepseek"
	ProviderMiniMax  = "minimax"
	ProviderMiMo     = "mimo"
	ProviderDoubao   = "doubao"
	ProviderQwen     = "qwen"
)

// ProviderLabels maps provider codes to Chinese display names.
var ProviderLabels = map[string]string{
	ProviderDeepSeek: "DeepSeek（深度求索）¥0.001/千tokens",
	ProviderMiniMax:  "MiniMax（海螺AI） ¥0.01/千tokens",
	ProviderMiMo:     "MiMo（小米）     ¥0.006/千tokens",
	ProviderDoubao:   "豆包（字节跳动） ¥0.008/千tokens",
	ProviderQwen:     "通义千问（阿里） ¥0.006/千tokens",
}

// Client is the interface every model provider must implement.
type Client interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Provider() string
}

// Config holds the parameters needed to create a model client.
type Config struct {
	Provider    string `yaml:"provider"     json:"provider"`
	APIKey      string `yaml:"api_key"      json:"api_key"`
	Endpoint    string `yaml:"endpoint"     json:"endpoint"`
	ModelName   string `yaml:"model_name"   json:"model_name"`
	MaxTokens   int    `yaml:"max_tokens"   json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	TopP        float64 `yaml:"top_p"        json:"top_p"`
	Timeout     time.Duration
	RetryTimes  int `yaml:"retry_times" json:"retry_times"`

	// Provider-specific extras
	GroupID      string `yaml:"group_id"       json:"group_id"`       // MiniMax
	EndpointID   string `yaml:"endpoint_id"    json:"endpoint_id"`    // Doubao (Volcengine)
	CompatibleMode bool  `yaml:"compatible_mode" json:"compatible_mode"` // Qwen: use OpenAI-compatible endpoint
}

// DefaultConfig returns a Config with sensible defaults for the given provider.
func DefaultConfig(provider string) Config {
	cfg := Config{
		Provider:    provider,
		MaxTokens:   4096,
		Temperature: 0.7,
		TopP:        0.9,
		Timeout:     60 * time.Second,
		RetryTimes:  3,
	}
	switch provider {
	case ProviderMiniMax:
		cfg.Endpoint = "https://api.minimax.chat/v1/text/chatcompletion_v2"
		cfg.ModelName = "abab6.5s-chat"
		cfg.Temperature = 0.8
	case ProviderDoubao:
		cfg.Endpoint = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
		cfg.ModelName = "doubao-pro-32k"
		cfg.MaxTokens = 8192
		cfg.Timeout = 90 * time.Second
	case ProviderQwen:
		cfg.Endpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
		cfg.ModelName = "qwen-long"
		cfg.MaxTokens = 6000
		cfg.Temperature = 0.75
		cfg.Timeout = 120 * time.Second
		cfg.CompatibleMode = true // default to compatible mode for simpler integration
	case ProviderDeepSeek:
		cfg.Endpoint = "https://api.deepseek.com/v1/chat/completions"
		cfg.ModelName = "deepseek-chat"
		cfg.Temperature = 0.6
		cfg.TopP = 0.95
	case ProviderMiMo:
		cfg.Endpoint = "https://token-plan-cn.xiaomimimo.com/v1/chat/completions"
		cfg.ModelName = "mimo-v2.5"
		cfg.Temperature = 0.7
		cfg.MaxTokens = 8192
	}
	return cfg
}

// --- Error types ---

type RetryExhaustedError struct {
	Provider string
	Err      error
}

func (e *RetryExhaustedError) Error() string {
	return fmt.Sprintf("model: %s retry exhausted: %v", e.Provider, e.Err)
}
func (e *RetryExhaustedError) Unwrap() error { return e.Err }

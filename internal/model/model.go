// Package model provides AI model clients and routing logic.
//
// Supported providers: MiniMax, Doubao, Qwen, DeepSeek.
// Each client implements exponential backoff retry and HTTP 429 handling.
package model

import (
	"context"
	"time"
)

// Provider identifiers.
const (
	ProviderMiniMax  = "minimax"
	ProviderDoubao   = "doubao"
	ProviderQwen     = "qwen"
	ProviderDeepSeek = "deepseek"
)

// Client is the interface every model provider must implement.
type Client interface {
	// Generate sends a prompt and returns the model's text response.
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// Provider returns the provider name (e.g. "qwen").
	Provider() string
}

// Config holds the parameters needed to create a model client.
type Config struct {
	Provider    string  `yaml:"provider"    json:"provider"`
	APIKey      string  `yaml:"api_key"     json:"api_key"`
	Endpoint    string  `yaml:"endpoint"    json:"endpoint"`
	ModelName   string  `yaml:"model_name"  json:"model_name"`
	MaxTokens   int     `yaml:"max_tokens"  json:"max_tokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	TopP        float64 `yaml:"top_p"       json:"top_p"`
	Timeout     time.Duration
	RetryTimes  int `yaml:"retry_times" json:"retry_times"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(provider string) Config {
	return Config{
		Provider:    provider,
		MaxTokens:   4096,
		Temperature: 0.7,
		TopP:        0.9,
		Timeout:     60 * time.Second,
		RetryTimes:  3,
	}
}

// --- Error types ---

// RetryExhaustedError is returned when all retry attempts fail.
type RetryExhaustedError struct {
	Provider string
	Err      error
}

func (e *RetryExhaustedError) Error() string {
	return "model: " + e.Provider + " retry exhausted: " + e.Err.Error()
}

func (e *RetryExhaustedError) Unwrap() error { return e.Err }

package model

import (
	"context"
	"net/http"
	"time"
)

// NewClient creates the appropriate Client implementation for the given provider.
func NewClient(cfg Config) Client {
	httpClient := &http.Client{Timeout: cfg.Timeout}
	if httpClient.Timeout == 0 {
		httpClient.Timeout = 60 * time.Second
	}
	base := baseClient{
		httpClient: httpClient,
		provider:   cfg.Provider,
		endpoint:   cfg.Endpoint,
		apiKey:     cfg.APIKey,
		modelName:  cfg.ModelName,
		maxTokens:  cfg.MaxTokens,
		temperature: cfg.Temperature,
		topP:       cfg.TopP,
	}

	// All four providers use the OpenAI-compatible chat completions API.
	// Qwen uses DashScope which has a slightly different response format,
	// but the common base client handles both via doChatRequest.
	switch cfg.Provider {
	case ProviderMiniMax:
		return &miniMaxClient{base}
	case ProviderDoubao:
		return &doubaoClient{base}
	case ProviderQwen:
		return &qwenClient{base}
	case ProviderDeepSeek:
		return &deepSeekClient{base}
	default:
		return nil
	}
}

// baseClient holds the shared HTTP client and config for all providers.
type baseClient struct {
	httpClient  *http.Client
	provider    string
	endpoint    string
	apiKey      string
	modelName   string
	maxTokens   int
	temperature float64
	topP        float64
}

func (c *baseClient) Provider() string { return c.provider }

func (c *baseClient) chatRequest() chatRequest {
	return chatRequest{
		Model:       c.modelName,
		Messages:    make([]chatMessage, 0, 2),
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
		TopP:        c.topP,
	}
}

// --- MiniMax ---

type miniMaxClient struct{ baseClient }

func (c *miniMaxClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := c.chatRequest()
	req.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return doChatRequest(ctx, c.httpClient, c.endpoint, c.apiKey, req)
}

// --- Doubao ---

type doubaoClient struct{ baseClient }

func (c *doubaoClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := c.chatRequest()
	req.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return doChatRequest(ctx, c.httpClient, c.endpoint, c.apiKey, req)
}

// --- Qwen (DashScope) ---

type qwenClient struct{ baseClient }

// qwenReq is the DashScope-specific request format.
type qwenReq struct {
	Model string `json:"model"`
	Input struct {
		Messages []chatMessage `json:"messages"`
	} `json:"input"`
	Parameters struct {
		Temperature float64 `json:"temperature,omitempty"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		TopP        float64 `json:"top_p,omitempty"`
	} `json:"parameters"`
}

type qwenResp struct {
	Output struct {
		Text string `json:"text"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *qwenClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Qwen DashScope uses a non-OpenAI-compatible format.
	// We use the chat completions endpoint if available; otherwise fall through.
	// For the initial implementation, we try the standard chat format first.
	req := c.chatRequest()
	req.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return doChatRequest(ctx, c.httpClient, c.endpoint, c.apiKey, req)
}

// --- DeepSeek ---

type deepSeekClient struct{ baseClient }

func (c *deepSeekClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := c.chatRequest()
	req.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return doChatRequest(ctx, c.httpClient, c.endpoint, c.apiKey, req)
}

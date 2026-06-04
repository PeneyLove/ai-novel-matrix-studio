package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewClient creates the appropriate Client for the given provider.
func NewClient(cfg Config) Client {
	httpClient := &http.Client{Timeout: cfg.Timeout}
	if httpClient.Timeout == 0 {
		httpClient.Timeout = 60 * time.Second
	}

	switch cfg.Provider {
	case ProviderMiniMax:
		return &miniMaxClient{httpClient: httpClient, cfg: cfg}
	case ProviderDoubao:
		return &doubaoClient{httpClient: httpClient, cfg: cfg}
	case ProviderQwen:
		if cfg.CompatibleMode {
			return &qwenCompatClient{httpClient: httpClient, cfg: cfg}
		}
		return &qwenNativeClient{httpClient: httpClient, cfg: cfg}
	case ProviderDeepSeek:
		return &deepSeekClient{httpClient: httpClient, cfg: cfg}
	default:
		return nil
	}
}

// ---- shared helpers ----

func doPost(ctx context.Context, client *http.Client, url, apiKey string, body []byte, extraHeaders map[string]string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("model: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("model: http request: %w", err)
	}
	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, fmt.Errorf("model: read response: %w", readErr)
	}
	return resp, respBody, nil
}

// ---- MiniMax (海螺AI) ----

type miniMaxClient struct {
	httpClient *http.Client
	cfg        Config
}

func (c *miniMaxClient) Provider() string { return ProviderMiniMax }

// miniMaxReq matches MiniMax's chatcompletion_v2 format.
type miniMaxReq struct {
	Model      string           `json:"model"`
	Messages   []msg            `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens  int              `json:"max_tokens,omitempty"`
	TopP       float64          `json:"top_p,omitempty"`
	ReplyConstraints struct {
		SenderType string `json:"sender_type"`
		SenderName string `json:"sender_name"`
	} `json:"reply_constraints"`
}

type msg struct {
	SenderType string `json:"sender_type"`
	Text       string `json:"text"`
}

type miniMaxResp struct {
	Reply    string `json:"reply"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func (c *miniMaxClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := miniMaxReq{
		Model:      c.cfg.ModelName,
		Temperature: c.cfg.Temperature,
		MaxTokens:  c.cfg.MaxTokens,
		TopP:       c.cfg.TopP,
	}
	req.ReplyConstraints.SenderType = "BOT"
	req.ReplyConstraints.SenderName = "智能助手"

	// MiniMax uses sender_type: USER / BOT instead of role: system / user
	if systemPrompt != "" {
		req.Messages = append(req.Messages, msg{SenderType: "SYSTEM", Text: systemPrompt})
	}
	req.Messages = append(req.Messages, msg{SenderType: "USER", Text: userPrompt})

	body, _ := json.Marshal(req)
	headers := map[string]string{}
	if c.cfg.GroupID != "" {
		headers["X-Minimax-Group-Id"] = c.cfg.GroupID
	}

	resp, respBody, err := doPost(ctx, c.httpClient, c.cfg.Endpoint, c.cfg.APIKey, body, headers)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(ProviderMiniMax, resp.StatusCode, respBody)
	}

	var mr miniMaxResp
	if err := json.Unmarshal(respBody, &mr); err != nil {
		return "", fmt.Errorf("minimax: parse response: %w", err)
	}
	if mr.BaseResp.StatusCode != 0 {
		return "", fmt.Errorf("minimax: API error [%d]: %s", mr.BaseResp.StatusCode, mr.BaseResp.StatusMsg)
	}
	return mr.Reply, nil
}

// ---- DeepSeek (深度求索) ----

type deepSeekClient struct {
	httpClient *http.Client
	cfg        Config
}

func (c *deepSeekClient) Provider() string { return ProviderDeepSeek }

// deepSeekReq is OpenAI-compatible with extra DeepSeek fields.
type deepSeekReq struct {
	Model            string    `json:"model"`
	Messages         []roleMsg `json:"messages"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
}

type roleMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsResp struct {
	Choices []struct {
		Message roleMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (c *deepSeekClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := deepSeekReq{
		Model:            c.cfg.ModelName,
		Temperature:      c.cfg.Temperature,
		MaxTokens:        c.cfg.MaxTokens,
		TopP:             c.cfg.TopP,
		FrequencyPenalty: 0.1, // slight repetition penalty
	}
	if systemPrompt != "" {
		req.Messages = append(req.Messages, roleMsg{Role: "system", Content: systemPrompt})
	}
	req.Messages = append(req.Messages, roleMsg{Role: "user", Content: userPrompt})

	body, _ := json.Marshal(req)
	resp, respBody, err := doPost(ctx, c.httpClient, c.cfg.Endpoint, c.cfg.APIKey, body, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(ProviderDeepSeek, resp.StatusCode, respBody)
	}

	var r dsResp
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("deepseek: parse response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("deepseek: API error [%s]: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("deepseek: empty response choices")
	}
	return r.Choices[0].Message.Content, nil
}

// ---- Qwen Native (DashScope) ----

type qwenNativeClient struct {
	httpClient *http.Client
	cfg        Config
}

func (c *qwenNativeClient) Provider() string { return ProviderQwen }

type qwenNativeReq struct {
	Model string `json:"model"`
	Input struct {
		Messages []roleMsg `json:"messages"`
	} `json:"input"`
	Parameters struct {
		Temperature     float64 `json:"temperature,omitempty"`
		MaxTokens       int     `json:"max_tokens,omitempty"`
		TopP            float64 `json:"top_p,omitempty"`
		ResultFormat    string  `json:"result_format,omitempty"`
		RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
	} `json:"parameters"`
}

type qwenNativeResp struct {
	Output struct {
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
		Choices      []struct {
			Message roleMsg `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (c *qwenNativeClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := qwenNativeReq{
		Model: c.cfg.ModelName,
	}
	req.Input.Messages = []roleMsg{}
	if systemPrompt != "" {
		req.Input.Messages = append(req.Input.Messages, roleMsg{Role: "system", Content: systemPrompt})
	}
	req.Input.Messages = append(req.Input.Messages, roleMsg{Role: "user", Content: userPrompt})
	req.Parameters.Temperature = c.cfg.Temperature
	req.Parameters.MaxTokens = c.cfg.MaxTokens
	req.Parameters.TopP = c.cfg.TopP
	req.Parameters.ResultFormat = "message"
	req.Parameters.RepetitionPenalty = 1.05

	body, _ := json.Marshal(req)
	resp, respBody, err := doPost(ctx, c.httpClient, c.cfg.Endpoint, c.cfg.APIKey, body, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(ProviderQwen, resp.StatusCode, respBody)
	}

	var qr qwenNativeResp
	if err := json.Unmarshal(respBody, &qr); err != nil {
		return "", fmt.Errorf("qwen: parse response: %w", err)
	}
	if qr.Code != "" && qr.Code != "0" {
		return "", fmt.Errorf("qwen: API error [%s]: %s", qr.Code, qr.Message)
	}
	// Prefer .choices[], fallback to .text
	if len(qr.Output.Choices) > 0 {
		return qr.Output.Choices[0].Message.Content, nil
	}
	if qr.Output.Text != "" {
		return qr.Output.Text, nil
	}
	return "", fmt.Errorf("qwen: empty response (finish_reason=%s)", qr.Output.FinishReason)
}

// ---- Qwen Compatible Mode (OpenAI-style) ----

type qwenCompatClient struct {
	httpClient *http.Client
	cfg        Config
}

func (c *qwenCompatClient) Provider() string { return ProviderQwen }

func (c *qwenCompatClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := deepSeekReq{
		Model:       c.cfg.ModelName,
		Temperature: c.cfg.Temperature,
		MaxTokens:   c.cfg.MaxTokens,
		TopP:        c.cfg.TopP,
		Stop:        []string{"\n\n\n"},
	}
	if systemPrompt != "" {
		req.Messages = append(req.Messages, roleMsg{Role: "system", Content: systemPrompt})
	}
	req.Messages = append(req.Messages, roleMsg{Role: "user", Content: userPrompt})

	body, _ := json.Marshal(req)
	resp, respBody, err := doPost(ctx, c.httpClient, c.cfg.Endpoint, c.cfg.APIKey, body, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(ProviderQwen, resp.StatusCode, respBody)
	}

	var r dsResp
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("qwen-compat: parse response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("qwen-compat: API error [%s]: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("qwen-compat: empty response choices")
	}
	return r.Choices[0].Message.Content, nil
}

// ---- Doubao (豆包 / Volcengine Ark) ----

type doubaoClient struct {
	httpClient *http.Client
	cfg        Config
}

func (c *doubaoClient) Provider() string { return ProviderDoubao }

func (c *doubaoClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// If endpoint_id is set, construct the Ark endpoint URL
	endpoint := c.cfg.Endpoint
	if c.cfg.EndpointID != "" && !strings.Contains(endpoint, "/endpoints/") {
		endpoint = fmt.Sprintf("https://ark.cn-beijing.volces.com/api/v3/endpoints/%s/chat/completions", c.cfg.EndpointID)
	}

	req := deepSeekReq{
		Model:       c.cfg.ModelName,
		Temperature: c.cfg.Temperature,
		MaxTokens:   c.cfg.MaxTokens,
		TopP:        c.cfg.TopP,
	}
	if systemPrompt != "" {
		req.Messages = append(req.Messages, roleMsg{Role: "system", Content: systemPrompt})
	}
	req.Messages = append(req.Messages, roleMsg{Role: "user", Content: userPrompt})

	body, _ := json.Marshal(req)
	resp, respBody, err := doPost(ctx, c.httpClient, endpoint, c.cfg.APIKey, body, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(ProviderDoubao, resp.StatusCode, respBody)
	}

	var r dsResp
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("doubao: parse response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("doubao: API error [%s]: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("doubao: empty response choices")
	}
	return r.Choices[0].Message.Content, nil
}

// ---- shared error parser ----

func parseAPIError(provider string, statusCode int, respBody []byte) error {
	bodyStr := string(respBody)
	// Truncate very long error bodies
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}
	if statusCode == http.StatusTooManyRequests {
		return &RateLimitError{Provider: provider, RetryAfter: 60 * time.Second}
	}
	return fmt.Errorf("%s: HTTP %d: %s", provider, statusCode, bodyStr)
}

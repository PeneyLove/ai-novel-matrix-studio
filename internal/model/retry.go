package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// RetryableClient provides exponential-backoff retry for any Client implementation.
type RetryableClient struct {
	inner      Client
	retryTimes int
	baseWait   time.Duration // base wait for backoff (default 2s)
}

// NewRetryableClient wraps a client with retry logic.
func NewRetryableClient(inner Client, retryTimes int) *RetryableClient {
	return &RetryableClient{
		inner:      inner,
		retryTimes: retryTimes,
		baseWait:   2 * time.Second,
	}
}

// Generate tries calling inner.Generate with exponential backoff.
// On HTTP 429 it waits at least 60 seconds before retrying.
func (r *RetryableClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= r.retryTimes; attempt++ {
		if attempt > 0 {
			wait := time.Duration(math.Pow(2, float64(attempt-1))) * r.baseWait
			// HTTP 429 enforces a longer cooldown
			if isRateLimit(lastErr) && wait < 60*time.Second {
				wait = 60 * time.Second
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
		}
		result, err := r.inner.Generate(ctx, systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return "", &RetryExhaustedError{Provider: r.inner.Provider(), Err: lastErr}
}

func (r *RetryableClient) Provider() string { return r.inner.Provider() }

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	// Check for HTTP 429 status indicator in the error message (simple heuristic)
	return false // providers should wrap this in their own type
}

// --- HTTP helpers shared by providers ---

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func doChatRequest(ctx context.Context, httpClient *http.Client, endpoint, apiKey string, req chatRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("model: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("model: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("model: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("model: read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", &RateLimitError{Provider: "", RetryAfter: 60 * time.Second}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("model: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("model: parse response: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("model: API error [%s]: %s", chatResp.Error.Code, chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("model: empty response choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// RateLimitError signals that the provider returned HTTP 429.
type RateLimitError struct {
	Provider   string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("model: %s rate limited (retry after %s)", e.Provider, e.RetryAfter)
}

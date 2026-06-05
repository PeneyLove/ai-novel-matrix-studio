package model

import (
	"context"
	"math"
	"strings"
	"time"
)

// RetryableClient provides exponential-backoff retry for any Client.
type RetryableClient struct {
	inner      Client
	retryTimes int
	baseWait   time.Duration
}

func NewRetryableClient(inner Client, retryTimes int) *RetryableClient {
	if retryTimes <= 0 {
		retryTimes = 3
	}
	return &RetryableClient{inner: inner, retryTimes: retryTimes, baseWait: 2 * time.Second}
}

// Generate tries inner.Generate with exponential backoff.
// HTTP 429 → waits at least 60s; other errors → 2^n seconds.
func (r *RetryableClient) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= r.retryTimes; attempt++ {
		if attempt > 0 {
			wait := time.Duration(math.Pow(2, float64(attempt-1))) * r.baseWait
			if isRateLimitErr(lastErr) && wait < 60*time.Second {
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

// StreamGenerate passes through streams directly — retries happen at the HTTP level.
func (r *RetryableClient) StreamGenerate(ctx context.Context, systemPrompt, userPrompt string) <-chan StreamChunk {
	return r.inner.StreamGenerate(ctx, systemPrompt, userPrompt)
}

// isRateLimitErr detects HTTP 429 / rate-limit errors across providers.
func isRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Direct type check
	if _, ok := err.(*RateLimitError); ok {
		return true
	}
	// Heuristic detection from error messages
	indicators := []string{
		"429", "rate limit", "RateLimit", "too many requests",
		"TooManyRequests", "frequency_limit", "请求过于频繁",
		"quota exceeded", "超出配额",
	}
	for _, ind := range indicators {
		if strings.Contains(msg, ind) {
			return true
		}
	}
	return false
}

// RateLimitError signals that the provider returned HTTP 429.
type RateLimitError struct {
	Provider   string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return "model: " + e.Provider + " rate limited (retry after " + e.RetryAfter.String() + ")"
}

// IsRateLimitError is a public helper for callers to check if an error is rate-limiting.
func IsRateLimitError(err error) bool { return isRateLimitErr(err) }

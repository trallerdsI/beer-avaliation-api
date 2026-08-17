package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Moderator defines the contract for content moderation checks.
type Moderator interface {
	IsContentAllowed(ctx context.Context, text string) (bool, error)
}

// OpenAIRequest represents the request payload for the OpenAI moderation API.
type OpenAIRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// OpenAIResponse represents the response from the OpenAI moderation API.
type OpenAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Results []struct {
		Flagged        bool               `json:"flagged"`
		Categories     map[string]bool    `json:"categories"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
}

// OpenAIModerator implements Moderator using the OpenAI Moderation API.
type OpenAIModerator struct {
	apiKey     string
	model      string
	httpClient *http.Client
	baseURL    string
}

// OpenAIModeratorOption is a functional option for configuring OpenAIModerator.
type OpenAIModeratorOption func(*OpenAIModerator)

// WithModel sets the moderation model name.
func WithModel(model string) OpenAIModeratorOption {
	return func(m *OpenAIModerator) {
		if strings.TrimSpace(model) != "" {
			m.model = model
		}
	}
}

// WithHTTPClient sets a custom HTTP client for the moderator.
func WithHTTPClient(client *http.Client) OpenAIModeratorOption {
	return func(m *OpenAIModerator) {
		if client != nil {
			m.httpClient = client
		}
	}
}

// WithBaseURL sets a custom base URL for the moderation API.
func WithBaseURL(baseURL string) OpenAIModeratorOption {
	return func(m *OpenAIModerator) {
		if strings.TrimSpace(baseURL) != "" {
			m.baseURL = strings.TrimRight(baseURL, "/")
		}
	}
}

// NewOpenAIModerator creates a new OpenAI moderator instance.
// apiKey is mandatory; returns error if empty.
func NewOpenAIModerator(apiKey string, opts ...OpenAIModeratorOption) (*OpenAIModerator, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	m := &OpenAIModerator{
		apiKey:  apiKey,
		model:   "omni-moderation-latest",
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 300 * time.Millisecond,
		},
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}

// IsContentAllowed checks if the given text passes OpenAI moderation.
// It follows a fail-open policy: if the API is unreachable or returns an error,
// the content is allowed and the error is logged.
func (m *OpenAIModerator) IsContentAllowed(ctx context.Context, text string) (bool, error) {
	if strings.TrimSpace(text) == "" {
		return true, nil
	}

	reqBody := OpenAIRequest{
		Model: m.model,
		Input: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, fmt.Errorf("failed to marshal moderation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/moderations", strings.NewReader(string(body)))
	if err != nil {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, fmt.Errorf("failed to create moderation request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, nil // fail-open: allow content on network errors
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, nil // fail-open: allow content on API errors
	}

	var moderationResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&moderationResp); err != nil {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, fmt.Errorf("failed to decode moderation response: %w", err)
	}

	if len(moderationResp.Results) == 0 {
		ModerationRequestsTotal.WithLabelValues("failed").Inc()
		return true, nil
	}

	result := moderationResp.Results[0]
	if result.Flagged {
		ModerationRequestsTotal.WithLabelValues("flagged").Inc()
		return false, nil
	}

	ModerationRequestsTotal.WithLabelValues("allowed").Inc()
	return true, nil
}

// NoopModerator implements Moderator by allowing all content without external calls.
type NoopModerator struct{}

// NewNoopModerator creates a new no-op moderator that always allows content.
func NewNoopModerator() *NoopModerator {
	return &NoopModerator{}
}

// IsContentAllowed always returns true for noop moderator.
func (m *NoopModerator) IsContentAllowed(_ context.Context, _ string) (bool, error) {
	ModerationRequestsTotal.WithLabelValues("allowed").Inc()
	return true, nil
}

// CachedOpenAIModerator wraps a Moderator with caching
// to reduce API calls for duplicate content.
type CachedOpenAIModerator struct {
	moderator Moderator
	cache     ModerationCache
}

// NewCachedOpenAIModerator creates a new cached moderator with the given cache backend.
func NewCachedOpenAIModerator(moderator Moderator, cache ModerationCache) *CachedOpenAIModerator {
	return &CachedOpenAIModerator{
		moderator: moderator,
		cache:     cache,
	}
}

// IsContentAllowed checks the cache first, then delegates to the underlying moderator.
func (m *CachedOpenAIModerator) IsContentAllowed(ctx context.Context, text string) (bool, error) {
	cached, found, err := m.cache.Get(ctx, text)
	if err != nil {
		return true, nil // fail-open on cache error
	}
	if found {
		return cached, nil
	}

	allowed, err := m.moderator.IsContentAllowed(ctx, text)
	if err != nil {
		return true, err // fail-open
	}

	if setErr := m.cache.Set(ctx, text, allowed); setErr != nil {
		return true, setErr // fail-open on cache set error
	}

	return allowed, nil
}

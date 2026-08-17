package moderation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModerator_IsContentAllowed_Flagged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "modr-123",
			"model": "omni-moderation-latest",
			"results": [
				{
					"flagged": true,
					"categories": {"hate": true},
					"category_scores": {"hate": 0.99}
				}
			]
		}`))
	}))
	defer server.Close()

	mod, err := NewOpenAIModerator("test-key", WithBaseURL(server.URL))
	require.NoError(t, err)

	allowed, err := mod.IsContentAllowed(context.Background(), "discurso de ódio")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestOpenAIModerator_IsContentAllowed_Allowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "modr-123",
			"model": "omni-moderation-latest",
			"results": [
				{
					"flagged": false,
					"categories": {"hate": false},
					"category_scores": {"hate": 0.01}
				}
			]
		}`))
	}))
	defer server.Close()

	mod, err := NewOpenAIModerator("test-key", WithBaseURL(server.URL))
	require.NoError(t, err)

	allowed, err := mod.IsContentAllowed(context.Background(), "cerveja artesanal")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestOpenAIModerator_IsContentAllowed_FailOpenOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mod, err := NewOpenAIModerator("test-key", WithBaseURL(server.URL), WithHTTPClient(&http.Client{Timeout: 300 * time.Millisecond}))
	require.NoError(t, err)

	allowed, err := mod.IsContentAllowed(context.Background(), "texto qualquer")
	require.NoError(t, err)
	assert.True(t, allowed, "fail-open: deve aprovar conteúdo quando a API não responde")
}

func TestOpenAIModerator_IsContentAllowed_FailOpenOnNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	mod, err := NewOpenAIModerator("test-key", WithBaseURL(server.URL))
	require.NoError(t, err)

	allowed, err := mod.IsContentAllowed(context.Background(), "texto qualquer")
	require.NoError(t, err)
	assert.True(t, allowed, "fail-open: deve aprovar conteúdo quando a API retorna erro")
}

func TestNoopModerator_AlwaysAllows(t *testing.T) {
	mod := NewNoopModerator()

	allowed, err := mod.IsContentAllowed(context.Background(), "qualquer texto")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestOpenAIModerator_EmptyInput(t *testing.T) {
	mod, err := NewOpenAIModerator("test-key")
	require.NoError(t, err)

	allowed, err := mod.IsContentAllowed(context.Background(), "   ")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestNewOpenAIModerator_EmptyAPIKey(t *testing.T) {
	_, err := NewOpenAIModerator("")
	assert.Error(t, err)
}

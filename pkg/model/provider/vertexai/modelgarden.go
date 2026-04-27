// Package vertexai provides support for non-Gemini models hosted on
// Google Cloud's Vertex AI Model Garden.
//
// Vertex AI Model Garden hosts models from various publishers (Anthropic,
// Meta, Mistral, etc.) and exposes them through two different APIs:
//
//   - Anthropic Claude models: the Anthropic-native `:rawPredict` /
//     `:streamRawPredict` endpoints. Claude models do not support the
//     OpenAI-compatible path.
//   - Other publishers: Vertex AI's OpenAI-compatible `/chat/completions`
//     endpoint.
//
// Authentication uses Google Cloud Application Default Credentials.
//
// Usage in agent config:
//
//	models:
//	  claude-on-vertex:
//	    provider: google
//	    model: claude-sonnet-4-20250514
//	    provider_opts:
//	      project: my-gcp-project
//	      location: us-east5
//	      publisher: anthropic
package vertexai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/environment"
	"github.com/docker/docker-agent/pkg/model/provider/anthropic"
	"github.com/docker/docker-agent/pkg/model/provider/base"
	"github.com/docker/docker-agent/pkg/model/provider/openai"
	"github.com/docker/docker-agent/pkg/model/provider/options"
	"github.com/docker/docker-agent/pkg/tools"
)

// cloudPlatformScope is the OAuth2 scope required for Vertex AI API access.
const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// validGCPIdentifier matches GCP project IDs and location names.
// Project IDs: 6-30 chars, lowercase letters, digits, hyphens.
// Locations: lowercase letters, digits, hyphens (e.g. us-central1).
var validGCPIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]{1,29}$`)

// Client is the subset of provider.Provider returned by NewClient. Both
// anthropic.Client and openai.Client satisfy it, so the caller can treat
// the two Model Garden code paths uniformly.
type Client interface {
	ID() string
	CreateChatCompletionStream(ctx context.Context, messages []chat.Message, tools []tools.Tool) (chat.MessageStream, error)
	BaseConfig() base.Config
}

// IsModelGardenConfig returns true when the ModelConfig describes a
// non-Gemini model on Vertex AI (i.e. the "publisher" provider_opt is set
// to something other than "google").
func IsModelGardenConfig(cfg *latest.ModelConfig) bool {
	p := publisher(cfg)
	return p != "" && !strings.EqualFold(p, "google")
}

// NewClient creates a client for a non-Gemini model on Vertex AI Model Garden,
// choosing the right endpoint based on the publisher:
//
//   - publisher: anthropic → Anthropic-native `:streamRawPredict` endpoint.
//   - other publishers → Vertex AI's OpenAI-compatible `/chat/completions`.
func NewClient(ctx context.Context, cfg *latest.ModelConfig, env environment.Provider, opts ...options.Opt) (Client, error) {
	project, location, err := resolveProjectLocation(ctx, cfg, env)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(publisher(cfg), "anthropic") {
		return anthropic.NewVertexClient(ctx, cfg, env, project, location, opts...)
	}
	return newOpenAIClient(ctx, cfg, env, project, location, opts...)
}

// publisher returns the provider_opts.publisher string, or "" if unset.
func publisher(cfg *latest.ModelConfig) string {
	if cfg == nil || cfg.ProviderOpts == nil {
		return ""
	}
	p, _ := cfg.ProviderOpts["publisher"].(string)
	return p
}

// resolveProjectLocation reads project and location from provider_opts, falls
// back to GOOGLE_CLOUD_PROJECT / GOOGLE_CLOUD_LOCATION, expands env var
// references, and validates the resulting values.
func resolveProjectLocation(ctx context.Context, cfg *latest.ModelConfig, env environment.Provider) (project, location string, err error) {
	if cfg == nil {
		return "", "", errors.New("model configuration is required")
	}

	project, _ = cfg.ProviderOpts["project"].(string)
	location, _ = cfg.ProviderOpts["location"].(string)

	if project, err = environment.Expand(ctx, project, env); err != nil {
		return "", "", fmt.Errorf("expanding project: %w", err)
	}
	if location, err = environment.Expand(ctx, location, env); err != nil {
		return "", "", fmt.Errorf("expanding location: %w", err)
	}

	if project == "" {
		project, _ = env.Get(ctx, "GOOGLE_CLOUD_PROJECT")
	}
	if location == "" {
		location, _ = env.Get(ctx, "GOOGLE_CLOUD_LOCATION")
	}

	if project == "" {
		return "", "", errors.New("vertex AI Model Garden requires a GCP project (set provider_opts.project or GOOGLE_CLOUD_PROJECT)")
	}
	if location == "" {
		return "", "", errors.New("vertex AI Model Garden requires a GCP location (set provider_opts.location or GOOGLE_CLOUD_LOCATION)")
	}

	// Validate to prevent URL path manipulation.
	if !validGCPIdentifier.MatchString(project) {
		return "", "", fmt.Errorf("invalid GCP project ID: %q", project)
	}
	if !validGCPIdentifier.MatchString(location) {
		return "", "", fmt.Errorf("invalid GCP location: %q", location)
	}

	return project, location, nil
}

// newOpenAIClient creates a client pointing at Vertex AI's OpenAI-compatible
// endpoint. It uses Google Application Default Credentials for authentication.
func newOpenAIClient(ctx context.Context, cfg *latest.ModelConfig, env environment.Provider, project, location string, opts ...options.Opt) (*openai.Client, error) {
	// https://cloud.google.com/vertex-ai/generative-ai/docs/partner-models/use-partner-models#openai_sdk
	baseURL := "https://" + location + "-aiplatform.googleapis.com/v1beta1/projects/" +
		url.PathEscape(project) + "/locations/" + url.PathEscape(location) + "/endpoints/openapi"

	slog.Debug("Creating Vertex AI Model Garden client",
		"publisher", publisher(cfg),
		"project", project,
		"location", location,
		"model", cfg.Model,
		"base_url", baseURL,
	)

	tokenSource, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain GCP credentials for Vertex AI: %w (run 'gcloud auth application-default login')", err)
	}
	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get GCP access token: %w", err)
	}

	// Build a config for the OpenAI provider with the Vertex base URL and a
	// synthetic token env var that the wrapping env provider resolves to a
	// fresh GCP access token.
	const tokenEnvVar = "_VERTEX_AI_ACCESS_TOKEN"
	oaiCfg := cfg.Clone()
	oaiCfg.BaseURL = baseURL
	oaiCfg.TokenKey = tokenEnvVar

	// Strip Vertex-specific provider_opts before handing off to the OpenAI
	// provider, and force the chat-completions API type.
	if oaiCfg.ProviderOpts == nil {
		oaiCfg.ProviderOpts = map[string]any{}
	}
	delete(oaiCfg.ProviderOpts, "project")
	delete(oaiCfg.ProviderOpts, "location")
	delete(oaiCfg.ProviderOpts, "publisher")
	oaiCfg.ProviderOpts["api_type"] = "openai_chatcompletions"

	wrappedEnv := &tokenEnv{
		Provider: env,
		key:      tokenEnvVar,
		tok:      token.AccessToken,
		ts:       tokenSource,
	}

	return openai.NewClient(ctx, oaiCfg, wrappedEnv, opts...)
}

// tokenEnv wraps an environment.Provider to inject a GCP access token,
// refreshing it on each Get call (TokenSource handles caching internally).
type tokenEnv struct {
	environment.Provider

	key string
	mu  sync.Mutex
	tok string
	ts  oauth2.TokenSource
}

func (e *tokenEnv) Get(ctx context.Context, name string) (string, bool) {
	if name != e.key {
		return e.Provider.Get(ctx, name)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	tok, err := e.ts.Token()
	if err != nil {
		slog.Warn("Failed to refresh GCP access token, using cached", "error", err)
		return e.tok, true
	}
	e.tok = tok.AccessToken
	return e.tok, true
}

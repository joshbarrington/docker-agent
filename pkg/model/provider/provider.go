// Package provider builds and dispatches to LLM provider clients.
//
// The package is organised across four files:
//
//   - provider.go (this file): the public Provider interfaces and the entry
//     points [New] and [NewWithModels] that callers use to construct a
//     provider from a model config.
//   - aliases.go: the built-in provider alias table (OpenAI-compatible
//     gateways such as ollama, mistral, xai, ...) and the helpers that expose
//     it to other packages without leaking the underlying map.
//   - defaults.go: pure config-merging logic that fills in defaults from
//     custom providers, built-in aliases, and model-specific rules
//     (thinking budget, interleaved thinking, ...).
//   - factory.go: dispatch from a resolved provider type to the concrete
//     client constructor (openai, anthropic, google, dmr, amazon-bedrock,
//     vertex AI), plus the rule-based router.
package provider

import (
	"context"
	"log/slog"

	"github.com/docker/docker-agent/pkg/chat"
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/environment"
	"github.com/docker/docker-agent/pkg/model/provider/base"
	"github.com/docker/docker-agent/pkg/model/provider/options"
	"github.com/docker/docker-agent/pkg/rag/types"
	"github.com/docker/docker-agent/pkg/tools"
)

// Provider defines the interface for model providers.
type Provider interface {
	// ID returns the model provider ID
	ID() string
	// CreateChatCompletionStream creates a streaming chat completion request.
	// It returns a stream that can be iterated over to get completion chunks.
	CreateChatCompletionStream(
		ctx context.Context,
		messages []chat.Message,
		tools []tools.Tool,
	) (chat.MessageStream, error)
	// BaseConfig returns the base configuration of this provider.
	BaseConfig() base.Config
}

// EmbeddingProvider defines the interface for providers that support embeddings.
type EmbeddingProvider interface {
	Provider
	// CreateEmbedding generates an embedding vector for the given text with usage tracking.
	CreateEmbedding(ctx context.Context, text string) (*base.EmbeddingResult, error)
}

// BatchEmbeddingProvider defines the interface for providers that support batch embeddings.
type BatchEmbeddingProvider interface {
	EmbeddingProvider
	// CreateBatchEmbedding generates embedding vectors for multiple texts with usage tracking.
	// Returns embeddings in the same order as input texts.
	CreateBatchEmbedding(ctx context.Context, texts []string) (*base.BatchEmbeddingResult, error)
}

// RerankingProvider defines the interface for providers that support reranking.
// Reranking models score query-document pairs to assess relevance.
type RerankingProvider interface {
	Provider
	// Rerank scores documents by relevance to the query.
	// Returns relevance scores in the same order as input documents.
	// Scores are typically in [0, 1] range where higher means more relevant.
	// criteria: Optional domain-specific guidance for relevance scoring (appended to base prompt)
	// documents: Array of types.Document with content and metadata
	Rerank(ctx context.Context, query string, documents []types.Document, criteria string) ([]float64, error)
}

// New creates a new provider from a model config.
// This is a convenience wrapper for NewWithModels with no models map.
func New(ctx context.Context, cfg *latest.ModelConfig, env environment.Provider, opts ...options.Opt) (Provider, error) {
	return NewWithModels(ctx, cfg, nil, env, opts...)
}

// NewWithModels creates a new provider from a model config with access to the full models map.
// The models map is used to resolve model references in routing rules.
func NewWithModels(ctx context.Context, cfg *latest.ModelConfig, models map[string]latest.ModelConfig, env environment.Provider, opts ...options.Opt) (Provider, error) {
	slog.Debug("Creating model provider", "type", cfg.Provider, "model", cfg.Model)

	// Check if this model has routing rules - if so, create a rule-based router
	if len(cfg.Routing) > 0 {
		return createRuleBasedRouter(ctx, cfg, models, env, opts...)
	}

	return createDirectProvider(ctx, cfg, env, opts...)
}

// createRuleBasedRouter creates a rule-based routing provider.
func createRuleBasedRouter(ctx context.Context, cfg *latest.ModelConfig, models map[string]latest.ModelConfig, env environment.Provider, opts ...options.Opt) (Provider, error) {
	// Create a provider factory that can resolve model references
	factory := func(ctx context.Context, modelSpec string, models map[string]latest.ModelConfig, env environment.Provider, factoryOpts ...options.Opt) (rulebased.Provider, error) {
		// Check if modelSpec is a reference to a model in the models map
		if modelCfg, exists := models[modelSpec]; exists {
			// Prevent infinite recursion - referenced models cannot have routing rules
			if len(modelCfg.Routing) > 0 {
				return nil, fmt.Errorf("model %q has routing rules and cannot be used as a routing target", modelSpec)
			}
			p, err := createDirectProvider(ctx, &modelCfg, env, factoryOpts...)
			if err != nil {
				return nil, err
			}
			return p, nil
		}

		// Otherwise, treat as an inline model spec (e.g., "openai/gpt-4o")
		inlineCfg, parseErr := latest.ParseModelRef(modelSpec)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid model spec %q: expected 'provider/model' format or a model reference", modelSpec)
		}
		p, err := createDirectProvider(ctx, &inlineCfg, env, factoryOpts...)
		if err != nil {
			return nil, err
		}
		return p, nil
	}

	return rulebased.NewClient(ctx, cfg, models, env, factory, opts...)
}

// createDirectProvider creates a provider without routing (direct model access).
func createDirectProvider(ctx context.Context, cfg *latest.ModelConfig, env environment.Provider, opts ...options.Opt) (Provider, error) {
	var globalOptions options.ModelOptions
	for _, opt := range opts {
		opt(&globalOptions)
	}

	// Apply defaults from custom providers (from config) or built-in aliases
	enhancedCfg := applyProviderDefaults(cfg, globalOptions.Providers())

	providerType := resolveProviderType(enhancedCfg)

	switch providerType {
	case "openai", "openai_chatcompletions", "openai_responses":
		return openai.NewClient(ctx, enhancedCfg, env, opts...)
	case "anthropic":
		return anthropic.NewClient(ctx, enhancedCfg, env, opts...)
	case "google":
		// Route non-Gemini models on Vertex AI (Model Garden) through the
		// OpenAI-compatible endpoint instead of the Gemini SDK.
		if vertexai.IsModelGardenConfig(enhancedCfg) {
			return vertexai.NewClient(ctx, enhancedCfg, env, opts...)
		}
		return gemini.NewClient(ctx, enhancedCfg, env, opts...)
	case "dmr":
		return dmr.NewClient(ctx, enhancedCfg, opts...)
	case "amazon-bedrock":
		return bedrock.NewClient(ctx, enhancedCfg, env, opts...)
	default:
		slog.Error("Unknown provider type", "type", providerType)
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// ---------------------------------------------------------------------------
// Provider-type resolution
// ---------------------------------------------------------------------------

// resolveProviderType determines the effective API type for a config.
// Priority: ProviderOpts["api_type"] > built-in alias > provider name.
func resolveProviderType(cfg *latest.ModelConfig) string {
	if cfg.ProviderOpts != nil {
		if apiType, ok := cfg.ProviderOpts["api_type"].(string); ok && apiType != "" {
			return apiType
		}
	}
	if alias, exists := Aliases[cfg.Provider]; exists && alias.APIType != "" {
		return alias.APIType
	}
	return cfg.Provider
}

// ---------------------------------------------------------------------------
// Provider defaults
// ---------------------------------------------------------------------------

// applyProviderDefaults applies default configuration from custom providers or built-in aliases.
// Custom providers (from config) take precedence over built-in aliases.
// This sets default base URLs, token keys, api_type, and model-specific defaults (like thinking budget).
//
// The returned config is a deep-enough copy: the caller's ModelConfig, ProviderOpts map,
// and ThinkingBudget pointer are never mutated.
func applyProviderDefaults(cfg *latest.ModelConfig, customProviders map[string]latest.ProviderConfig) *latest.ModelConfig {
	// Create a copy to avoid modifying the original.
	// cloneModelConfig also deep-copies ProviderOpts so writes are safe.
	enhancedCfg := cloneModelConfig(cfg)

	if customProviders != nil {
		if providerCfg, exists := customProviders[cfg.Provider]; exists {
			slog.Debug("Applying custom provider defaults",
				"provider", cfg.Provider,
				"model", cfg.Model,
				"base_url", providerCfg.BaseURL,
			)

			// Apply the underlying provider type if set on the provider config.
			// This allows the model to inherit the real provider type (e.g., "anthropic")
			// so that the correct API client is selected.
			if providerCfg.Provider != "" {
				enhancedCfg.Provider = providerCfg.Provider
			}

			if enhancedCfg.BaseURL == "" && providerCfg.BaseURL != "" {
				enhancedCfg.BaseURL = providerCfg.BaseURL
			}
			if enhancedCfg.TokenKey == "" && providerCfg.TokenKey != "" {
				enhancedCfg.TokenKey = providerCfg.TokenKey
			}
			if enhancedCfg.Temperature == nil && providerCfg.Temperature != nil {
				enhancedCfg.Temperature = providerCfg.Temperature
			}
			if enhancedCfg.MaxTokens == nil && providerCfg.MaxTokens != nil {
				enhancedCfg.MaxTokens = providerCfg.MaxTokens
			}
			if enhancedCfg.TopP == nil && providerCfg.TopP != nil {
				enhancedCfg.TopP = providerCfg.TopP
			}
			if enhancedCfg.FrequencyPenalty == nil && providerCfg.FrequencyPenalty != nil {
				enhancedCfg.FrequencyPenalty = providerCfg.FrequencyPenalty
			}
			if enhancedCfg.PresencePenalty == nil && providerCfg.PresencePenalty != nil {
				enhancedCfg.PresencePenalty = providerCfg.PresencePenalty
			}
			if enhancedCfg.ParallelToolCalls == nil && providerCfg.ParallelToolCalls != nil {
				enhancedCfg.ParallelToolCalls = providerCfg.ParallelToolCalls
			}
			if enhancedCfg.TrackUsage == nil && providerCfg.TrackUsage != nil {
				enhancedCfg.TrackUsage = providerCfg.TrackUsage
			}
			if enhancedCfg.ThinkingBudget == nil && providerCfg.ThinkingBudget != nil {
				enhancedCfg.ThinkingBudget = providerCfg.ThinkingBudget
			}

			// Merge provider_opts from provider config (model opts take precedence)
			if len(providerCfg.ProviderOpts) > 0 {
				if enhancedCfg.ProviderOpts == nil {
					enhancedCfg.ProviderOpts = make(map[string]any)
				}
				for k, v := range providerCfg.ProviderOpts {
					if _, has := enhancedCfg.ProviderOpts[k]; !has {
						enhancedCfg.ProviderOpts[k] = v
					}
				}
			}

			// Set api_type in ProviderOpts if not already set.
			// Only default to openai_chatcompletions for OpenAI-compatible providers.
			if providerCfg.APIType != "" {
				if enhancedCfg.ProviderOpts == nil {
					enhancedCfg.ProviderOpts = make(map[string]any)
				}
				if _, has := enhancedCfg.ProviderOpts["api_type"]; !has {
					enhancedCfg.ProviderOpts["api_type"] = providerCfg.APIType
				}
			} else if isOpenAICompatibleProvider(resolveEffectiveProvider(providerCfg)) {
				if enhancedCfg.ProviderOpts == nil {
					enhancedCfg.ProviderOpts = make(map[string]any)
				}
				if _, has := enhancedCfg.ProviderOpts["api_type"]; !has {
					enhancedCfg.ProviderOpts["api_type"] = "openai_chatcompletions"
				}
			}

			applyModelDefaults(enhancedCfg)
			return enhancedCfg
		}
	}

	if alias, exists := Aliases[cfg.Provider]; exists {
		// Set default base URL if not already specified
		if enhancedCfg.BaseURL == "" && alias.BaseURL != "" {
			enhancedCfg.BaseURL = alias.BaseURL
		}

		// Set default token key if not already specified
		if enhancedCfg.TokenKey == "" && alias.TokenEnvVar != "" {
			enhancedCfg.TokenKey = alias.TokenEnvVar
		}
	}

	// Apply model-specific defaults (e.g., thinking budget for Claude/GPT models)
	applyModelDefaults(enhancedCfg)
	return enhancedCfg
}

// ---------------------------------------------------------------------------
// Thinking defaults and overrides
// ---------------------------------------------------------------------------

// applyModelDefaults applies provider-specific default values for model configuration.
//
// Thinking defaults policy:
//   - thinking_budget: 0  or  thinking_budget: none  →  thinking is off (nil).
//   - thinking_budget explicitly set to a real value  →  kept as-is; interleaved_thinking
//     is auto-enabled for Anthropic/Bedrock-Claude.
//   - thinking_budget NOT set:
//   - Thinking-only models (OpenAI o-series) get "medium".
//   - All other models get no thinking.
//
// NOTE: max_tokens is NOT set here; see teamloader and runtime/model_switcher.
func applyModelDefaults(cfg *latest.ModelConfig) {
	// Set appropriate github copilot api_type.
	applyGithubCopilotAPIType(cfg)

	// Explicitly disabled → normalise to nil so providers never see it.
	if cfg.ThinkingBudget.IsDisabled() {
		cfg.ThinkingBudget = nil
		slog.Debug("Thinking explicitly disabled",
			"provider", cfg.Provider, "model", cfg.Model)
		return
	}

	providerType := resolveProviderType(cfg)

	// User already set a real thinking_budget — just apply side-effects.
	if cfg.ThinkingBudget != nil {
		ensureInterleavedThinking(cfg, providerType)
		return
	}

	// No thinking_budget configured — only thinking-only models get a default.
	switch providerType {
	case "openai", "openai_chatcompletions", "openai_responses":
		if isOpenAIThinkingOnlyModel(cfg.Model) {
			cfg.ThinkingBudget = &latest.ThinkingBudget{Effort: "medium"}
			slog.Debug("Applied default thinking for thinking-only OpenAI model",
				"provider", cfg.Provider, "model", cfg.Model)
		}
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// cloneModelConfig returns a shallow copy of cfg with a deep copy of
// ProviderOpts so that callers can safely mutate the returned config's
// map and pointer fields without affecting the original.
func cloneModelConfig(cfg *latest.ModelConfig) *latest.ModelConfig {
	c := *cfg
	if cfg.ProviderOpts != nil {
		c.ProviderOpts = make(map[string]any, len(cfg.ProviderOpts))
		maps.Copy(c.ProviderOpts, cfg.ProviderOpts)
	}
	return &c
}

// ensureInterleavedThinking sets interleaved_thinking=true in ProviderOpts
// for Anthropic and Bedrock-Claude models, unless the user already set it.
func ensureInterleavedThinking(cfg *latest.ModelConfig, providerType string) {
	needsInterleaved := providerType == "anthropic" ||
		(providerType == "amazon-bedrock" && isBedrockClaudeModel(cfg.Model))
	if !needsInterleaved {
		return
	}
	if cfg.ProviderOpts == nil {
		cfg.ProviderOpts = make(map[string]any)
	}
	if _, has := cfg.ProviderOpts["interleaved_thinking"]; !has {
		cfg.ProviderOpts["interleaved_thinking"] = true
		slog.Debug("Auto-enabled interleaved_thinking",
			"provider", cfg.Provider, "model", cfg.Model)
	}
}

// applyGithubCopilotAPIType ensures api_type is set to openai_responses for appropriate models.
func applyGithubCopilotAPIType(cfg *latest.ModelConfig) {
	if isGithubCopilotProvider(cfg.Provider) && isCopilotResponsesModel(cfg.Model) {
		if cfg.ProviderOpts == nil {
			cfg.ProviderOpts = make(map[string]any)
		}
		// If it's not set, or was set to openai_chatcompletions by the generic fallback, override it.
		if apiType, ok := cfg.ProviderOpts["api_type"].(string); !ok || apiType == "" || apiType == "openai_chatcompletions" {
			cfg.ProviderOpts["api_type"] = "openai_responses"
		}
	}
}

// isOpenAIThinkingOnlyModel returns true for OpenAI models that require thinking
// to function properly (o-series reasoning models).
func isOpenAIThinkingOnlyModel(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "o1") ||
		strings.HasPrefix(m, "o3") ||
		strings.HasPrefix(m, "o4")
}

// isBedrockClaudeModel returns true if the model ID is a Claude model on Bedrock.
// Claude model IDs on Bedrock start with "anthropic.claude-" or "global.anthropic.claude-".
func isBedrockClaudeModel(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "anthropic.claude-") || strings.HasPrefix(m, "global.anthropic.claude-")
}

// gemini3Family extracts the model family (e.g. "pro", "flash") from a
// Gemini 3+ model name, or returns "" if the model is not Gemini 3+.
// It handles both "gemini-3-<family>" and "gemini-3.X-<family>" patterns.
//
// Examples:
//
//	gemini3Family("gemini-3-pro")              → "pro"
//	gemini3Family("gemini-3.1-flash-preview")  → "flash-preview"
//	gemini3Family("gemini-2.5-flash")          → ""
func gemini3Family(model string) string {
	if !strings.HasPrefix(model, "gemini-3") {
		return ""
	}
	rest := model[len("gemini-3"):]
	if rest == "" {
		return ""
	}
	// Accept "gemini-3-..." or "gemini-3.X-..." (e.g. gemini-3.1-pro-preview)
	switch rest[0] {
	case '-':
		return rest[1:] // "gemini-3-pro" → "pro"
	case '.':
		if _, family, ok := strings.Cut(rest, "-"); ok {
			return family // "gemini-3.1-pro-preview" → "pro-preview"
		}
	}
	return ""
}

func isGeminiProModel(model string) bool {
	return strings.HasPrefix(gemini3Family(model), "pro")
}

func isGeminiFlashModel(model string) bool {
	return strings.HasPrefix(gemini3Family(model), "flash")
}

// resolveEffectiveProvider returns the effective provider type for a ProviderConfig.
// If Provider is explicitly set, returns that. Otherwise returns "openai" (backward compat).
func resolveEffectiveProvider(cfg latest.ProviderConfig) string {
	if cfg.Provider != "" {
		return cfg.Provider
	}
	return "openai"
}

func isGithubCopilotProvider(providerType string) bool {
	switch providerType {
	case "github-copilot":
		return true
	default:
		return false
	}
}

func isCopilotResponsesModel(model string) bool {
	codex := map[string]bool{
		"gpt-5.3-codex": true,
		"gpt-5.2-codex": true,
	}
	return codex[model]
}

// isOpenAICompatibleProvider returns true if the provider type uses the OpenAI API protocol.
func isOpenAICompatibleProvider(providerType string) bool {
	switch providerType {
	case "openai", "openai_chatcompletions", "openai_responses":
		return true
	default:
		// Check if it's an alias that maps to openai
		if alias, exists := Aliases[providerType]; exists {
			return alias.APIType == "openai"
		}
		return false
	}
}

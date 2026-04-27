package provider

import (
	"cmp"
	"log/slog"
	"maps"
	"strings"

	"github.com/docker/docker-agent/pkg/config/latest"
)

// ---------------------------------------------------------------------------
// Provider-type resolution
// ---------------------------------------------------------------------------

// resolveProviderType determines the effective API type for a config.
// Priority: ProviderOpts["api_type"] > built-in alias > provider name.
// Reading from a nil ProviderOpts map is safe and yields the zero value.
func resolveProviderType(cfg *latest.ModelConfig) string {
	if apiType, ok := cfg.ProviderOpts["api_type"].(string); ok && apiType != "" {
		return apiType
	}
	if alias, exists := LookupAlias(cfg.Provider); exists && alias.APIType != "" {
		return alias.APIType
	}
	return cfg.Provider
}

// resolveEffectiveProvider returns the effective provider type for a ProviderConfig.
// If Provider is explicitly set, returns that. Otherwise returns "openai" (backward compat).
func resolveEffectiveProvider(cfg latest.ProviderConfig) string {
	return cmp.Or(cfg.Provider, "openai")
}

// isOpenAICompatibleProvider returns true if the provider type uses the OpenAI API protocol.
func isOpenAICompatibleProvider(providerType string) bool {
	switch providerType {
	case "openai", "openai_chatcompletions", "openai_responses":
		return true
	}
	// Otherwise, the type is OpenAI-compatible iff it's an alias that maps to OpenAI.
	alias, exists := LookupAlias(providerType)
	return exists && alias.APIType == "openai"
}

// ---------------------------------------------------------------------------
// Provider defaults
// ---------------------------------------------------------------------------

// applyProviderDefaults applies default configuration from custom providers or built-in aliases.
// Custom providers (from config) take precedence over built-in aliases.
// This sets default base URLs, token keys, api_type, and model-specific defaults (like thinking budget).
//
// The returned config is a deep-enough copy: the caller's ModelConfig, ProviderOpts map,
// and ThinkingBudget/TaskBudget pointers are never mutated.
func applyProviderDefaults(cfg *latest.ModelConfig, customProviders map[string]latest.ProviderConfig) *latest.ModelConfig {
	// Create a copy to avoid modifying the original.
	// cloneModelConfig also deep-copies ProviderOpts so writes are safe.
	enhancedCfg := cloneModelConfig(cfg)

	if providerCfg, exists := customProviders[cfg.Provider]; exists {
		slog.Debug("Applying custom provider defaults",
			"provider", cfg.Provider,
			"model", cfg.Model,
			"base_url", providerCfg.BaseURL,
		)
		mergeFromProviderConfig(enhancedCfg, providerCfg)
		applyModelDefaults(enhancedCfg)
		return enhancedCfg
	}

	if alias, exists := LookupAlias(cfg.Provider); exists {
		applyAliasFallbacks(enhancedCfg, alias)
	}

	// Apply model-specific defaults (e.g., thinking budget for Claude/GPT models)
	applyModelDefaults(enhancedCfg)
	return enhancedCfg
}

// mergeFromProviderConfig merges defaults from a custom ProviderConfig into a
// ModelConfig. Model-level fields always take precedence; provider-level fields
// only fill in unset (nil/empty) fields. The destination ProviderOpts map is
// assumed to be safe to mutate (cloneModelConfig copies it).
func mergeFromProviderConfig(dst *latest.ModelConfig, src latest.ProviderConfig) {
	// Apply the underlying provider type if set on the provider config.
	// This allows the model to inherit the real provider type (e.g., "anthropic")
	// so that the correct API client is selected.
	if src.Provider != "" {
		dst.Provider = src.Provider
	}

	if dst.BaseURL == "" {
		dst.BaseURL = src.BaseURL
	}
	if dst.TokenKey == "" {
		dst.TokenKey = src.TokenKey
	}
	setIfNil(&dst.Temperature, src.Temperature)
	setIfNil(&dst.MaxTokens, src.MaxTokens)
	setIfNil(&dst.TopP, src.TopP)
	setIfNil(&dst.FrequencyPenalty, src.FrequencyPenalty)
	setIfNil(&dst.PresencePenalty, src.PresencePenalty)
	setIfNil(&dst.ParallelToolCalls, src.ParallelToolCalls)
	setIfNil(&dst.TrackUsage, src.TrackUsage)
	setIfNil(&dst.ThinkingBudget, src.ThinkingBudget)
	setIfNil(&dst.TaskBudget, src.TaskBudget)

	// Merge provider_opts from provider config (model opts take precedence).
	for k, v := range src.ProviderOpts {
		setProviderOptIfAbsent(dst, k, v)
	}

	// Set api_type in ProviderOpts if not already set.
	// Prefer the explicit APIType from the provider config; otherwise default to
	// openai_chatcompletions for OpenAI-compatible providers.
	switch {
	case src.APIType != "":
		setProviderOptIfAbsent(dst, "api_type", src.APIType)
	case isOpenAICompatibleProvider(resolveEffectiveProvider(src)):
		setProviderOptIfAbsent(dst, "api_type", "openai_chatcompletions")
	}
}

// applyAliasFallbacks applies BaseURL and TokenKey defaults from a built-in
// alias when the model config does not already specify them.
func applyAliasFallbacks(dst *latest.ModelConfig, alias Alias) {
	if dst.BaseURL == "" {
		dst.BaseURL = alias.BaseURL
	}
	if dst.TokenKey == "" {
		dst.TokenKey = alias.TokenEnvVar
	}
}

// setIfNil assigns src to *dst when *dst is nil. It centralises the repetitive
// "only fill in if unset" pattern used when merging provider-level defaults.
func setIfNil[T any](dst **T, src *T) {
	if *dst == nil && src != nil {
		*dst = src
	}
}

// setProviderOptIfAbsent stores key=value in cfg.ProviderOpts unless the key is
// already set. The map is created lazily.
func setProviderOptIfAbsent(cfg *latest.ModelConfig, key string, value any) {
	if cfg.ProviderOpts == nil {
		cfg.ProviderOpts = make(map[string]any)
	}
	if _, has := cfg.ProviderOpts[key]; !has {
		cfg.ProviderOpts[key] = value
	}
}

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

// ---------------------------------------------------------------------------
// Model-name predicates
// ---------------------------------------------------------------------------

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

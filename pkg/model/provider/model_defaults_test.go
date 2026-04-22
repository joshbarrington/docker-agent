package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/config/latest"
)

func TestApplyModelDefaults(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name            string
		config          *latest.ModelConfig
		wantBudget      *latest.ThinkingBudget // nil means no thinking
		wantInterleaved *bool                  // nil means key must not exist
	}{
		// --- OpenAI: only o-series gets defaults ---
		{
			name:   "openai/gpt-4o: no default thinking",
			config: &latest.ModelConfig{Provider: "openai", Model: "gpt-4o"},
		},
		{
			name:   "openai/gpt-5: no default thinking",
			config: &latest.ModelConfig{Provider: "openai", Model: "gpt-5"},
		},
		{
			name:       "openai/o3-mini: thinking-only model gets default",
			config:     &latest.ModelConfig{Provider: "openai", Model: "o3-mini"},
			wantBudget: &latest.ThinkingBudget{Effort: "medium"},
		},
		{
			name:       "openai/o1: thinking-only model gets default",
			config:     &latest.ModelConfig{Provider: "openai", Model: "o1"},
			wantBudget: &latest.ThinkingBudget{Effort: "medium"},
		},
		{
			name:       "openai/o4-mini: thinking-only model gets default",
			config:     &latest.ModelConfig{Provider: "openai", Model: "o4-mini"},
			wantBudget: &latest.ThinkingBudget{Effort: "medium"},
		},
		{
			name:       "openai/o3-mini: explicit budget overrides default",
			config:     &latest.ModelConfig{Provider: "openai", Model: "o3-mini", ThinkingBudget: &latest.ThinkingBudget{Effort: "high"}},
			wantBudget: &latest.ThinkingBudget{Effort: "high"},
		},
		{
			name:       "openai/gpt-4o: explicit budget preserved",
			config:     &latest.ModelConfig{Provider: "openai", Model: "gpt-4o", ThinkingBudget: &latest.ThinkingBudget{Effort: "high"}},
			wantBudget: &latest.ThinkingBudget{Effort: "high"},
		},

		// --- Aliases (resolve to openai) — no default thinking ---
		{
			name:   "mistral: no default thinking",
			config: &latest.ModelConfig{Provider: "mistral", Model: "mistral-large-latest"},
		},
		{
			name:   "xai: no default thinking",
			config: &latest.ModelConfig{Provider: "xai", Model: "grok-2"},
		},
		{
			name:   "custom openai_chatcompletions: no default thinking",
			config: &latest.ModelConfig{Provider: "custom", Model: "custom-model", ProviderOpts: map[string]any{"api_type": "openai_chatcompletions"}},
		},

		// --- Anthropic: no default, but interleaved_thinking when budget set ---
		{
			name:   "anthropic: no default thinking",
			config: &latest.ModelConfig{Provider: "anthropic", Model: "claude-sonnet-4-0"},
		},
		{
			name:            "anthropic: explicit budget enables interleaved_thinking",
			config:          &latest.ModelConfig{Provider: "anthropic", Model: "claude-sonnet-4-0", ThinkingBudget: &latest.ThinkingBudget{Tokens: 16384}},
			wantBudget:      &latest.ThinkingBudget{Tokens: 16384},
			wantInterleaved: boolPtr(true),
		},
		{
			name:            "anthropic: adaptive budget enables interleaved_thinking",
			config:          &latest.ModelConfig{Provider: "anthropic", Model: "claude-opus-4-6", ThinkingBudget: &latest.ThinkingBudget{Effort: "adaptive"}},
			wantBudget:      &latest.ThinkingBudget{Effort: "adaptive"},
			wantInterleaved: boolPtr(true),
		},
		{
			name:            "anthropic: explicit interleaved_thinking=false is preserved",
			config:          &latest.ModelConfig{Provider: "anthropic", Model: "claude-sonnet-4-0", ThinkingBudget: &latest.ThinkingBudget{Tokens: 8192}, ProviderOpts: map[string]any{"interleaved_thinking": false}},
			wantBudget:      &latest.ThinkingBudget{Tokens: 8192},
			wantInterleaved: boolPtr(false),
		},

		// --- Google: no default thinking ---
		{
			name:   "google/gemini-2.5-flash: no default thinking",
			config: &latest.ModelConfig{Provider: "google", Model: "gemini-2.5-flash"},
		},
		{
			name:   "google/gemini-3-pro: no default thinking",
			config: &latest.ModelConfig{Provider: "google", Model: "gemini-3-pro"},
		},
		{
			name:       "google: explicit budget preserved",
			config:     &latest.ModelConfig{Provider: "google", Model: "gemini-2.5-flash", ThinkingBudget: &latest.ThinkingBudget{Tokens: 8192}},
			wantBudget: &latest.ThinkingBudget{Tokens: 8192},
		},

		// --- Bedrock: no default thinking, interleaved_thinking when budget set on Claude ---
		{
			name:   "bedrock claude: no default thinking",
			config: &latest.ModelConfig{Provider: "amazon-bedrock", Model: "anthropic.claude-3-sonnet"},
		},
		{
			name:   "bedrock global claude: no default thinking",
			config: &latest.ModelConfig{Provider: "amazon-bedrock", Model: "global.anthropic.claude-sonnet-4-5-20250929-v1:0"},
		},
		{
			name:            "bedrock claude: explicit budget enables interleaved_thinking",
			config:          &latest.ModelConfig{Provider: "amazon-bedrock", Model: "anthropic.claude-3-sonnet", ThinkingBudget: &latest.ThinkingBudget{Tokens: 8192}},
			wantBudget:      &latest.ThinkingBudget{Tokens: 8192},
			wantInterleaved: boolPtr(true),
		},
		{
			name:   "bedrock non-claude: not affected",
			config: &latest.ModelConfig{Provider: "amazon-bedrock", Model: "amazon.titan-text-express-v1"},
		},

		// --- Disabled thinking normalised to nil ---
		{
			name:   "thinking_budget: 0 becomes nil",
			config: &latest.ModelConfig{Provider: "anthropic", Model: "claude-sonnet-4-0", ThinkingBudget: &latest.ThinkingBudget{Tokens: 0}},
		},
		{
			name:   "thinking_budget: none becomes nil",
			config: &latest.ModelConfig{Provider: "openai", Model: "gpt-4o", ThinkingBudget: &latest.ThinkingBudget{Effort: "none"}},
		},

		// --- Unknown / other providers: no effect ---
		{
			name:   "unknown provider: no effect",
			config: &latest.ModelConfig{Provider: "unknown", Model: "some-model"},
		},
		{
			name:   "dmr: no effect",
			config: &latest.ModelConfig{Provider: "dmr", Model: "ai/llama3.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			applyModelDefaults(tt.config)

			// Check thinking budget.
			if tt.wantBudget == nil {
				assert.Nil(t, tt.config.ThinkingBudget)
			} else {
				require.NotNil(t, tt.config.ThinkingBudget)
				assert.Equal(t, *tt.wantBudget, *tt.config.ThinkingBudget)
			}

			// Check interleaved_thinking.
			if tt.wantInterleaved == nil {
				if tt.config.ProviderOpts != nil {
					_, exists := tt.config.ProviderOpts["interleaved_thinking"]
					assert.False(t, exists, "interleaved_thinking should not be set")
				}
			} else {
				require.NotNil(t, tt.config.ProviderOpts)
				assert.Equal(t, *tt.wantInterleaved, tt.config.ProviderOpts["interleaved_thinking"])
			}
		})
	}
}

func TestApplyProviderDefaults(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name            string
		config          *latest.ModelConfig
		customProviders map[string]latest.ProviderConfig
		wantBudget      *latest.ThinkingBudget
		wantInterleaved *bool
	}{
		{
			name:       "openai o3-mini: thinking-only gets default through provider defaults",
			config:     &latest.ModelConfig{Provider: "openai", Model: "o3-mini"},
			wantBudget: &latest.ThinkingBudget{Effort: "medium"},
		},
		{
			name:   "openai gpt-4o: no default through provider defaults",
			config: &latest.ModelConfig{Provider: "openai", Model: "gpt-4o"},
		},
		{
			name:            "anthropic with explicit budget gets interleaved through provider defaults",
			config:          &latest.ModelConfig{Provider: "anthropic", Model: "claude-sonnet-4-0", ThinkingBudget: &latest.ThinkingBudget{Tokens: 8192}},
			wantBudget:      &latest.ThinkingBudget{Tokens: 8192},
			wantInterleaved: boolPtr(true),
		},
		{
			name:   "custom openai provider: no default thinking",
			config: &latest.ModelConfig{Provider: "my_gateway", Model: "gpt-4o"},
			customProviders: map[string]latest.ProviderConfig{
				"my_gateway": {APIType: "openai_chatcompletions", BaseURL: "https://api.example.com/v1", TokenKey: "MY_KEY"},
			},
		},
		{
			name:       "explicit thinking preserved unchanged",
			config:     &latest.ModelConfig{Provider: "openai", Model: "gpt-4o", ThinkingBudget: &latest.ThinkingBudget{Effort: "high"}},
			wantBudget: &latest.ThinkingBudget{Effort: "high"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := applyProviderDefaults(tt.config, tt.customProviders)

			if tt.wantBudget == nil {
				assert.Nil(t, result.ThinkingBudget)
			} else {
				require.NotNil(t, result.ThinkingBudget)
				assert.Equal(t, *tt.wantBudget, *result.ThinkingBudget)
			}

			if tt.wantInterleaved != nil {
				require.NotNil(t, result.ProviderOpts)
				assert.Equal(t, *tt.wantInterleaved, result.ProviderOpts["interleaved_thinking"])
			}
		})
	}
}

func TestIsOpenAIThinkingOnlyModel(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		model string
		want  bool
	}{
		{"o1", true},
		{"o1-preview", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-4o", false},
		{"gpt-4.1", false},
		{"gpt-5", false},
		{"custom-model", false},
	} {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isOpenAIThinkingOnlyModel(tt.model))
		})
	}
}

// TestApplyProviderDefaults_DoesNotModifyOriginal verifies that applyProviderDefaults
// does not mutate the input config's ProviderOpts map.
func TestApplyProviderDefaults_DoesNotModifyOriginal(t *testing.T) {
	t.Parallel()

	original := &latest.ModelConfig{
		Provider:       "anthropic",
		Model:          "claude-sonnet-4-0",
		ThinkingBudget: &latest.ThinkingBudget{Tokens: 8192},
		ProviderOpts:   map[string]any{"custom_key": "original_value"},
	}

	result := applyProviderDefaults(original, nil)

	// Result should have interleaved_thinking set (because thinking_budget is set).
	require.NotNil(t, result.ProviderOpts)
	assert.Equal(t, true, result.ProviderOpts["interleaved_thinking"])

	// Original must NOT have interleaved_thinking added.
	_, exists := original.ProviderOpts["interleaved_thinking"]
	assert.False(t, exists, "original ProviderOpts must not be mutated by applyProviderDefaults")

	// Original custom key must still be there.
	assert.Equal(t, "original_value", original.ProviderOpts["custom_key"])
}

func TestIsCopilotResponsesModel(t *testing.T) {
	t.Parallel()

	assert.True(t, isCopilotResponsesModel("gpt-5.3-codex"))
	assert.True(t, isCopilotResponsesModel("gpt-5.2-codex"))
	assert.False(t, isCopilotResponsesModel("gpt-4o"))
	assert.False(t, isCopilotResponsesModel("claude-sonnet-4-5"))
	assert.False(t, isCopilotResponsesModel(""))
}

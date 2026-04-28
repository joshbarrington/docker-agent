package v6

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillsConfig_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SkillsConfig
	}{
		{
			name:     "boolean true",
			input:    "true",
			expected: SkillsConfig{Sources: []string{"local"}},
		},
		{
			name:     "boolean false",
			input:    "false",
			expected: SkillsConfig{Sources: nil},
		},
		{
			name:     "list with local only",
			input:    "[local]",
			expected: SkillsConfig{Sources: []string{"local"}},
		},
		{
			name:     "list with remote URL",
			input:    "[\"http://example.com\"]",
			expected: SkillsConfig{Sources: []string{"http://example.com"}},
		},
		{
			name:  "list with local and remote",
			input: "[local, \"https://skills.example.com\"]",
			expected: SkillsConfig{Sources: []string{
				"local",
				"https://skills.example.com",
			}},
		},
		{
			name: "multiline list",
			input: `- local
- https://example.com
- http://internal.corp`,
			expected: SkillsConfig{Sources: []string{
				"local",
				"https://example.com",
				"http://internal.corp",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg SkillsConfig
			err := yaml.Unmarshal([]byte(tt.input), &cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestSkillsConfig_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    SkillsConfig
		expected string
	}{
		{
			name:     "disabled",
			input:    SkillsConfig{},
			expected: "false\n",
		},
		{
			name:     "local only marshals as true",
			input:    SkillsConfig{Sources: []string{"local"}},
			expected: "true\n",
		},
		{
			name:     "list with remote",
			input:    SkillsConfig{Sources: []string{"local", "https://example.com"}},
			expected: "- local\n- https://example.com\n",
		},
		{
			name:     "remote only",
			input:    SkillsConfig{Sources: []string{"https://example.com"}},
			expected: "- https://example.com\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := yaml.Marshal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(out))
		})
	}
}

func TestSkillsConfig_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SkillsConfig
	}{
		{
			name:     "boolean true",
			input:    "true",
			expected: SkillsConfig{Sources: []string{"local"}},
		},
		{
			name:     "boolean false",
			input:    "false",
			expected: SkillsConfig{Sources: nil},
		},
		{
			name:     "list with local",
			input:    `["local"]`,
			expected: SkillsConfig{Sources: []string{"local"}},
		},
		{
			name:     "list with remote URLs",
			input:    `["local", "https://skills.example.com"]`,
			expected: SkillsConfig{Sources: []string{"local", "https://skills.example.com"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg SkillsConfig
			err := json.Unmarshal([]byte(tt.input), &cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestSkillsConfig_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    SkillsConfig
		expected string
	}{
		{
			name:     "disabled",
			input:    SkillsConfig{},
			expected: "false",
		},
		{
			name:     "local only as true",
			input:    SkillsConfig{Sources: []string{"local"}},
			expected: "true",
		},
		{
			name:     "list with remote",
			input:    SkillsConfig{Sources: []string{"local", "https://example.com"}},
			expected: `["local","https://example.com"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := json.Marshal(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(out))
		})
	}
}

func TestSkillsConfig_Enabled(t *testing.T) {
	assert.False(t, SkillsConfig{}.Enabled())
	assert.False(t, SkillsConfig{Sources: nil}.Enabled())
	assert.False(t, SkillsConfig{Sources: []string{}}.Enabled())
	assert.True(t, SkillsConfig{Sources: []string{"local"}}.Enabled())
	assert.True(t, SkillsConfig{Sources: []string{"https://example.com"}}.Enabled())
}

func TestSkillsConfig_HasLocal(t *testing.T) {
	assert.False(t, SkillsConfig{}.HasLocal())
	assert.False(t, SkillsConfig{Sources: []string{"https://example.com"}}.HasLocal())
	assert.True(t, SkillsConfig{Sources: []string{"local"}}.HasLocal())
	assert.True(t, SkillsConfig{Sources: []string{"local", "https://example.com"}}.HasLocal())
}

func TestSkillsConfig_RemoteURLs(t *testing.T) {
	assert.Empty(t, SkillsConfig{}.RemoteURLs())
	assert.Empty(t, SkillsConfig{Sources: []string{"local"}}.RemoteURLs())
	assert.Equal(t,
		[]string{"https://example.com", "http://internal.corp"},
		SkillsConfig{Sources: []string{"local", "https://example.com", "http://internal.corp"}}.RemoteURLs(),
	)
}

func TestSkillsConfig_JSONRoundTrip(t *testing.T) {
	// This tests the upgrade path from v4 (bool) to v5 (SkillsConfig) via CloneThroughJSON
	t.Run("bool true round trips through JSON", func(t *testing.T) {
		jsonData := []byte("true")
		var cfg SkillsConfig
		require.NoError(t, json.Unmarshal(jsonData, &cfg))
		assert.True(t, cfg.Enabled())
		assert.True(t, cfg.HasLocal())
		assert.Equal(t, []string{"local"}, cfg.Sources)

		out, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, "true", string(out))
	})

	t.Run("bool false round trips through JSON", func(t *testing.T) {
		jsonData := []byte("false")
		var cfg SkillsConfig
		require.NoError(t, json.Unmarshal(jsonData, &cfg))
		assert.False(t, cfg.Enabled())
		assert.Nil(t, cfg.Sources)

		out, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, "false", string(out))
	})

	t.Run("list round trips through JSON", func(t *testing.T) {
		jsonData := []byte(`["local","https://example.com"]`)
		var cfg SkillsConfig
		require.NoError(t, json.Unmarshal(jsonData, &cfg))
		assert.True(t, cfg.Enabled())
		assert.Equal(t, []string{"local", "https://example.com"}, cfg.Sources)

		out, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, `["local","https://example.com"]`, string(out))
	})
}

func TestSkillsConfig_InAgentConfig(t *testing.T) {
	yamlInput := `
model: openai/gpt-4
skills:
  - local
  - https://skills.example.com
toolsets:
  - type: filesystem
`
	var agent AgentConfig
	err := yaml.Unmarshal([]byte(yamlInput), &agent)
	require.NoError(t, err)
	assert.True(t, agent.Skills.Enabled())
	assert.True(t, agent.Skills.HasLocal())
	assert.Equal(t, []string{"https://skills.example.com"}, agent.Skills.RemoteURLs())
}

func TestSkillsConfig_InAgentConfigBool(t *testing.T) {
	yamlInput := `
model: openai/gpt-4
skills: true
toolsets:
  - type: filesystem
`
	var agent AgentConfig
	err := yaml.Unmarshal([]byte(yamlInput), &agent)
	require.NoError(t, err)
	assert.True(t, agent.Skills.Enabled())
	assert.True(t, agent.Skills.HasLocal())
	assert.Empty(t, agent.Skills.RemoteURLs())
}

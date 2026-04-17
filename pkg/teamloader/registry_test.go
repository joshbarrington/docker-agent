package teamloader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker-agent/pkg/config"
	"github.com/docker/docker-agent/pkg/config/latest"
	"github.com/docker/docker-agent/pkg/environment"
	"github.com/docker/docker-agent/pkg/tools"
)

func TestCreateShellTool(t *testing.T) {
	toolset := latest.Toolset{
		Type: "shell",
	}

	registry := NewDefaultToolsetRegistry()

	runConfig := &config.RuntimeConfig{
		Config:              config.Config{WorkingDir: t.TempDir()},
		EnvProviderForTests: environment.NewOsEnvProvider(),
	}

	tool, err := registry.CreateTool(t.Context(), toolset, ".", runConfig, "test-agent")
	require.NoError(t, err)
	require.NotNil(t, tool)
}

func TestCreateMCPTool_CommandNotFound_CreatesToolsetAnyway(t *testing.T) {
	t.Setenv("DOCKER_AGENT_TOOLS_DIR", t.TempDir())

	toolset := latest.Toolset{
		Type:    "mcp",
		Command: "./bin/nonexistent-mcp-server",
	}

	registry := NewDefaultToolsetRegistry()

	runConfig := &config.RuntimeConfig{
		Config:              config.Config{WorkingDir: t.TempDir()},
		EnvProviderForTests: environment.NewOsEnvProvider(),
	}

	tool, err := registry.CreateTool(t.Context(), toolset, ".", runConfig, "test-agent")
	require.NoError(t, err)
	require.NotNil(t, tool)
	assert.Equal(t, "mcp(stdio cmd=./bin/nonexistent-mcp-server)", tools.DescribeToolSet(tool))
}

func TestCreateMCPTool_BareCommandNotFound_CreatesToolsetAnyway(t *testing.T) {
	t.Setenv("DOCKER_AGENT_TOOLS_DIR", t.TempDir())

	toolset := latest.Toolset{
		Type:    "mcp",
		Command: "some-nonexistent-mcp-binary",
	}

	registry := NewDefaultToolsetRegistry()

	runConfig := &config.RuntimeConfig{
		Config:              config.Config{WorkingDir: t.TempDir()},
		EnvProviderForTests: environment.NewOsEnvProvider(),
	}

	tool, err := registry.CreateTool(t.Context(), toolset, ".", runConfig, "test-agent")
	require.NoError(t, err)
	require.NotNil(t, tool)
	assert.Equal(t, "mcp(stdio cmd=some-nonexistent-mcp-binary)", tools.DescribeToolSet(tool))
}

func TestResolveToolsetWorkingDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		toolsetWorkingDir string
		agentWorkingDir   string
		want              string
	}{
		{
			name:              "empty toolset dir returns agent dir",
			toolsetWorkingDir: "",
			agentWorkingDir:   "/workspace",
			want:              "/workspace",
		},
		{
			name:              "absolute toolset dir is returned as-is",
			toolsetWorkingDir: "/tmp/mcp",
			agentWorkingDir:   "/workspace",
			want:              "/tmp/mcp",
		},
		{
			name:              "relative toolset dir is joined with agent dir",
			toolsetWorkingDir: "./backend",
			agentWorkingDir:   "/workspace",
			want:              "/workspace/backend",
		},
		{
			name:              "relative toolset dir without leading dot is joined with agent dir",
			toolsetWorkingDir: "tools/mcp",
			agentWorkingDir:   "/workspace",
			want:              "/workspace/tools/mcp",
		},
		{
			name:              "relative toolset dir with empty agent dir returns toolset dir unchanged",
			toolsetWorkingDir: "./backend",
			agentWorkingDir:   "",
			want:              "./backend",
		},
		{
			name:              "both empty returns empty",
			toolsetWorkingDir: "",
			agentWorkingDir:   "",
			want:              "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveToolsetWorkingDir(tt.toolsetWorkingDir, tt.agentWorkingDir)
			assert.Equal(t, tt.want, got)
		})
	}
}

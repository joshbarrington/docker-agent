package latest

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestToolset_Validate_LSP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "valid lsp with command",
			config: `
version: "3"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
        command: gopls
`,
			wantErr: "",
		},
		{
			name: "lsp missing command",
			config: `
version: "3"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
`,
			wantErr: "lsp toolset requires a command to be set",
		},
		{
			name: "lsp with args",
			config: `
version: "3"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
        command: gopls
        args:
          - -remote=auto
`,
			wantErr: "",
		},
		{
			name: "lsp with env",
			config: `
version: "3"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
        command: gopls
        env:
          GOFLAGS: "-mod=vendor"
`,
			wantErr: "",
		},
		{
			name: "lsp with file_types",
			config: `
version: "5"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
        command: gopls
        file_types: [".go", ".mod"]
`,
			wantErr: "",
		},
		{
			name: "file_types on non-lsp toolset",
			config: `
version: "5"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: shell
        file_types: [".go"]
`,
			wantErr: "file_types can only be used with type 'lsp'",
		},
		{
			name: "lsp with working_dir",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: lsp
        command: gopls
        working_dir: ./backend
`,
			wantErr: "",
		},
		{
			name: "working_dir on non-mcp-lsp toolset is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: shell
        working_dir: ./backend
`,
			wantErr: "working_dir can only be used with type 'mcp' or 'lsp'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg Config
			err := yaml.Unmarshal([]byte(tt.config), &cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestToolset_Validate_MCP_WorkingDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    string
		wantErr   string
		wantValue string
	}{
		{
			name: "mcp with working_dir",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        command: my-mcp-server
        working_dir: ./tools/mcp
`,
			wantErr:   "",
			wantValue: "./tools/mcp",
		},
		{
			name: "mcp without working_dir defaults to empty",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        command: my-mcp-server
`,
			wantErr:   "",
			wantValue: "",
		},
		{
			name: "working_dir on remote mcp is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
        working_dir: ./tools
`,
			wantErr:   "working_dir is not valid for remote MCP toolsets",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg Config
			err := yaml.Unmarshal([]byte(tt.config), &cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantValue, cfg.Agents.First().Toolsets[0].WorkingDir)
			}
		})
	}
}

func TestToolset_Validate_MCP_RemoteOAuth_CallbackRedirectURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "callbackRedirectURL absolute URL is accepted",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: https://redirect.example.com/cb
`,
			wantErr: "",
		},
		{
			name: "callbackRedirectURL with placeholder is accepted",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "https://redirect.example.com/cb?port=${callbackPort}"
`,
			wantErr: "",
		},
		{
			name: "http on loopback is accepted",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "http://localhost:${callbackPort}/cb"
`,
			wantErr: "",
		},
		{
			name: "http on non-loopback host is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "http://redirect.example.com/cb"
`,
			wantErr: "must use https for non-loopback hosts",
		},
		{
			name: "javascript scheme is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "javascript:alert(1)"
`,
			wantErr: "must be an absolute URL",
		},
		{
			name: "ftp scheme is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "ftp://example.com/cb"
`,
			wantErr: "scheme must be http or https",
		},
		{
			name: "relative callbackRedirectURL is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: /just/a/path
`,
			wantErr: "oauth callbackRedirectURL must be an absolute URL",
		},
		{
			name: "garbage callbackRedirectURL is rejected",
			config: `
version: "8"
agents:
  root:
    model: "openai/gpt-4"
    toolsets:
      - type: mcp
        remote:
          url: https://mcp.example.com/sse
          oauth:
            clientId: cid
            callbackRedirectURL: "://bad-url"
`,
			wantErr: "oauth callbackRedirectURL must be an absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg Config
			err := yaml.Unmarshal([]byte(tt.config), &cfg)

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

---
title: "Tool Configuration"
description: "Complete reference for configuring built-in tools, MCP tools, and Docker-based tools."
permalink: /configuration/tools/
---

# Tool Configuration

_Complete reference for configuring built-in tools, MCP tools, and Docker-based tools._

## Built-in Tools

Built-in tools are included with docker-agent and require no external dependencies. Add them to your agent's `toolsets` list by `type`. Each tool's dedicated page covers its full configuration options, available operations, and examples.

| Type | Description | Page |
| --- | --- | --- |
| `filesystem` | Read, write, list, search, navigate | [Filesystem]({{ '/tools/filesystem/' | relative_url }}) |
| `shell` | Execute shell commands | [Shell]({{ '/tools/shell/' | relative_url }}) |
| `think` | Reasoning scratchpad | [Think]({{ '/tools/think/' | relative_url }}) |
| `todo` | Task list management | [Todo]({{ '/tools/todo/' | relative_url }}) |
| `memory` | Persistent key-value storage (SQLite) | [Memory]({{ '/tools/memory/' | relative_url }}) |
| `fetch` | HTTP requests | [Fetch]({{ '/tools/fetch/' | relative_url }}) |
| `script` | Custom shell scripts as tools | [Script]({{ '/tools/script/' | relative_url }}) |
| `lsp` | Language Server Protocol integration | [LSP]({{ '/tools/lsp/' | relative_url }}) |
| `api` | Custom HTTP API tools | [API]({{ '/tools/api/' | relative_url }}) |
| `user_prompt` | Interactive user input | [User Prompt]({{ '/tools/user-prompt/' | relative_url }}) |
| `transfer_task` | Delegate to sub-agents (auto-enabled) | [Transfer Task]({{ '/tools/transfer-task/' | relative_url }}) |
| `background_agents` | Parallel sub-agent dispatch | [Background Agents]({{ '/tools/background-agents/' | relative_url }}) |
| `handoff` | A2A remote agent delegation | [Handoff]({{ '/tools/handoff/' | relative_url }}) |
| `a2a` | A2A remote agent connection | [A2A]({{ '/tools/a2a/' | relative_url }}) |

**Example:**

```yaml
toolsets:
  - type: filesystem
  - type: shell
  - type: think
  - type: todo
  - type: memory
    path: ./dev.db
```

## MCP Tools

Extend agents with external tools via the [Model Context Protocol](https://modelcontextprotocol.io/).

### Docker MCP (Recommended)

Run MCP servers as secure Docker containers via the [MCP Gateway](https://github.com/docker/mcp-gateway):

```yaml
toolsets:
  - type: mcp
    ref: docker:duckduckgo # web search
  - type: mcp
    ref: docker:github-official # GitHub integration
```

Browse available tools at the [Docker MCP Catalog](https://hub.docker.com/search?q=&type=mcp).

| Property      | Type   | Description                                                      |
| ------------- | ------ | ---------------------------------------------------------------- |
| `ref`         | string | Docker MCP reference (`docker:name`)                             |
| `tools`       | array  | Optional: only expose these tools                                |
| `instruction` | string | Custom instructions injected into the agent's context            |
| `config`      | any    | MCP server-specific configuration (passed during initialization) |
| `working_dir` | string | Working directory for the MCP gateway subprocess. Only applies when the catalog entry runs as a local process (not remote). Relative paths are resolved against the agent's working directory. |

### Local MCP (stdio)

Run MCP servers as local processes communicating over stdin/stdout:

```yaml
toolsets:
  - type: mcp
    command: python
    args: ["-m", "mcp_server"]
    tools: ["search", "fetch"]
    env:
      API_KEY: value
```

| Property | Type | Description |
| --- | --- | --- |
| `command` | string | Command to execute the MCP server |
| `args` | array | Command arguments |
| `tools` | array | Optional: only expose these tools |
| `env` | object | Environment variables (key-value pairs) |
| `working_dir` | string | Working directory for the MCP server process. Relative paths are resolved against the agent's working directory. Defaults to the agent's working directory when omitted. |
| `instruction` | string | Custom instructions injected into the agent's context |
| `version` | string | Package reference for [auto-installing](#auto-installing-tools) the command binary |

### Remote MCP (SSE / Streamable HTTP)

Connect to MCP servers over the network:

```yaml
toolsets:
  - type: mcp
    remote:
      url: "https://mcp-server.example.com"
      transport_type: "sse"
      headers:
        Authorization: "Bearer your-token"
    tools: ["search_web", "fetch_url"]
```

| Property                | Type   | Description                       |
| ----------------------- | ------ | --------------------------------- |
| `remote.url`            | string | Base URL of the MCP server        |
| `remote.transport_type` | string | `sse` or `streamable`             |
| `remote.headers`        | object | HTTP headers (typically for auth) |

## Auto-Installing Tools

When configuring MCP or LSP tools that require a binary command, docker agent can **automatically download and install** the command if it's not already available on your system. This uses the [aqua registry](https://github.com/aquaproj/aqua-registry) — a curated index of CLI tool packages.

### How It Works

1. When a toolset with a `command` is loaded, docker agent checks if the command is available in your `PATH`
2. If not found, it checks the docker agent tools directory (`~/.cagent/tools/bin/`)
3. If still not found, it looks up the command in the aqua registry and installs it automatically

### Explicit Package Reference

Use the `version` property to specify exactly which package to install:

```yaml
toolsets:
  - type: mcp
    command: gopls
    version: "golang/tools@v0.21.0"
    args: ["mcp"]
  - type: lsp
    command: rust-analyzer
    version: "rust-lang/rust-analyzer@2024-01-01"
    file_types: [".rs"]
```

The format is `owner/repo` or `owner/repo@version`. When a version is omitted, the latest release is used.

### Automatic Detection

If the `version` property is not set, docker agent tries to auto-detect the package from the command name by searching the aqua registry:

```yaml
toolsets:
  - type: mcp
    command: gopls  # auto-detected as golang/tools
    args: ["mcp"]
```

### Disabling Auto-Install

**Per toolset** — set `version` to `"false"` or `"off"`:

```yaml
toolsets:
  - type: mcp
    command: my-custom-server
    version: "false"
```

**Globally** — set the `DOCKER_AGENT_AUTO_INSTALL` environment variable:

```bash
export DOCKER_AGENT_AUTO_INSTALL=false
```

### Environment Variables

| Variable                     | Default            | Description                                      |
| ---------------------------- | ------------------ | ------------------------------------------------ |
| `DOCKER_AGENT_AUTO_INSTALL`  | (enabled)          | Set to `false` to disable all auto-installation  |
| `DOCKER_AGENT_TOOLS_DIR`     | `~/.cagent/tools/` | Base directory for installed tools               |
| `GITHUB_TOKEN`               | —                  | GitHub token to raise API rate limits (optional) |

Installed binaries are placed in `~/.cagent/tools/bin/` and cached so they are only downloaded once.

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 Tip
</div>
  <p>Auto-install supports both Go packages (via <code>go install</code>) and GitHub release binaries (via archive download). The aqua registry metadata determines which method is used.</p>
</div>

## Tool Filtering

Toolsets may expose many tools. Use the `tools` property to whitelist only the ones your agent needs. This works for any toolset type — not just MCP:

```yaml
toolsets:
  - type: mcp
    ref: docker:github-official
    tools: ["list_issues", "create_issue", "get_pull_request"]
  - type: filesystem
    tools: ["read_file", "search_files_content"]
  - type: shell
    tools: ["shell"]
```

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 Tip
</div>
  <p>Filtering tools improves agent performance — fewer tools means less confusion for the model about which tool to use.</p>
</div>

## Tool Instructions

Add context-specific instructions that get injected when a toolset is loaded:

```yaml
toolsets:
  - type: mcp
    ref: docker:github-official
    instruction: |
      Use these tools to manage GitHub issues.
      Always check for existing issues before creating new ones.
      Label new issues with 'triage' by default.
```

## Deferred Tool Loading

Load tools on-demand to speed up agent startup:

```yaml
toolsets:
  - type: mcp
    ref: docker:github-official
    defer: true
  - type: mcp
    ref: docker:slack
    defer: true
  - type: filesystem
```

Or defer specific tools within a toolset:

```yaml
toolsets:
  - type: mcp
    ref: docker:github-official
    defer:
      - "list_issues"
      - "search_repos"
```

## Combined Example

```yaml
agents:
  root:
    model: anthropic/claude-sonnet-4-0
    description: Full-featured developer assistant
    instruction: You are an expert developer.
    toolsets:
      # Built-in tools
      - type: filesystem
      - type: shell
      - type: think
      - type: todo
      - type: memory
        path: ./dev.db
      - type: user_prompt
      # LSP for code intelligence
      - type: lsp
        command: gopls
        file_types: [".go"]
      # Custom scripts
      - type: script
        shell:
          run_tests:
            description: Run the test suite
            cmd: mise test
          lint:
            description: Run the linter
            cmd: mise lint
      # Custom API tool
      - type: api
        api_config:
          name: get_status
          method: GET
          endpoint: "https://api.example.com/status"
          instruction: Check service health
      # Docker MCP tools
      - type: mcp
        ref: docker:github-official
        tools: ["list_issues", "create_issue"]
      - type: mcp
        ref: docker:duckduckgo
      # Remote MCP
      - type: mcp
        remote:
          url: "https://internal-api.example.com/mcp"
          transport_type: "sse"
          headers:
            Authorization: "Bearer ${INTERNAL_TOKEN}"
```

<div class="callout callout-warning" markdown="1">
<div class="callout-title">⚠️ Toolset Order Matters
</div>
  <p>If multiple toolsets provide a tool with the same name, the first one wins. Order your toolsets intentionally.</p>
</div>

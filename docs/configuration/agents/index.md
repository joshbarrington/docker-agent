---
title: "Agent Configuration"
description: "Complete reference for defining agents in your YAML configuration."
permalink: /configuration/agents/
---

# Agent Configuration

_Complete reference for defining agents in your YAML configuration._

## Full Schema

<!-- yaml-lint:skip -->
```yaml
agents:
  agent_name:
    model: string # Required: model reference
    description: string # Required: what this agent does
    instruction: string # Required: system prompt
    sub_agents: [list] # Optional: local or external sub-agent references
    toolsets: [list] # Optional: tool configurations (use `type: rag` for RAG sources)
    fallback: # Optional: fallback config
      models: [list]
      retries: 2
      cooldown: 1m
    add_date: boolean # Optional: add date to context
    add_environment_info: boolean # Optional: add env info to context
    add_prompt_files: [list] # Optional: include additional prompt files
    add_description_parameter: bool # Optional: add description to tool schema
    code_mode_tools: boolean # Optional: enable code mode tool format
    max_iterations: int # Optional: max tool-calling loops
    max_consecutive_tool_calls: int # Optional: max identical consecutive tool calls
    max_old_tool_call_tokens: int # Optional: token budget for old tool call content
    num_history_items: int # Optional: limit conversation history
    skills: boolean | [list] # Optional: enable skill discovery (true/false or list of names and/or sources)
    commands: # Optional: named prompts
      name: "prompt text"
    welcome_message: string # Optional: message shown at session start
    handoffs: [list] # Optional: agent names this agent can hand off to
    hooks: # Optional: lifecycle hooks
      pre_tool_use: [list]
      post_tool_use: [list]
      session_start: [list]
      session_end: [list]
      on_user_input: [list]
      stop: [list]
      notification: [list]
    structured_output: # Optional: constrain output format
      name: string
      schema: object
```

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 See also
</div>
  <p>For model parameters, see <a href="{{ '/configuration/models/' | relative_url }}">Model Config</a>. For tool details, see <a href="{{ '/configuration/tools/' | relative_url }}">Tool Config</a>. For multi-agent patterns, see <a href="{{ '/concepts/multi-agent/' | relative_url }}">Multi-Agent</a>.</p>

</div>

## Properties Reference

| Property                    | Type    | Required | Description                                                                                                                                                                   |
| --------------------------- | ------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `model`                     | string  | ✓        | Model reference. Either inline (`openai/gpt-4o`) or a named model from the `models` section.                                                                                  |
| `description`               | string  | ✓        | Brief description of the agent's purpose. Used by coordinators to decide delegation.                                                                                          |
| `instruction`               | string  | ✓        | System prompt that defines the agent's behavior, personality, and constraints.                                                                                                |
| `sub_agents`                | array   | ✗        | List of agent names or external OCI references this agent can delegate to. Supports local agents, registry references (e.g., `agentcatalog/pirate`), and named references (`name:reference`). Automatically enables the `transfer_task` tool. See [External Sub-Agents]({{ '/concepts/multi-agent/#external-sub-agents-from-registries' | relative_url }}). |
| `toolsets`                  | array   | ✗        | List of tool configurations. See [Tool Config]({{ '/configuration/tools/' | relative_url }}).                                                                                                        |
| `fallback`                  | object  | ✗        | Automatic model failover configuration.                                                                                                                                       |
| `add_date`                  | boolean | ✗        | When `true`, injects the current date into the agent's context.                                                                                                               |
| `add_environment_info`      | boolean | ✗        | When `true`, injects working directory, OS, CPU architecture, and git info into context.                                                                                      |
| `add_prompt_files`          | array   | ✗        | List of file paths whose contents are appended to the system prompt. Useful for including coding standards, guidelines, or additional context.                                |
| `add_description_parameter` | boolean | ✗        | When `true`, adds agent descriptions as a parameter in tool schemas. Helps with tool selection in multi-agent scenarios.                                                      |
| `code_mode_tools`           | boolean | ✗        | When `true`, formats tool responses in a code-optimized format with structured output schemas. Useful for MCP gateway and programmatic access.                                |
| `max_iterations`            | int     | ✗        | Maximum number of tool-calling loops. Default: unlimited (0). Set this to prevent infinite loops.                                                                             |
| `max_consecutive_tool_calls` | int     | ✗        | Maximum consecutive identical tool calls before the agent is terminated, preventing degenerate loops. Default: `5`.                                                          |
| `max_old_tool_call_tokens`  | int     | ✗        | Maximum number of tokens to keep from old tool call arguments and results. Older tool calls beyond this budget have their content replaced with a placeholder, saving context space. Tokens are approximated as `len/4`. Set to `-1` to disable truncation (unlimited). Default: `40000`. |
| `num_history_items`         | int     | ✗        | Limit the number of conversation history messages sent to the model. Useful for managing context window size with long conversations. Default: unlimited (all messages sent). |
| `rag`                       | array   | ✗        | List of RAG source names to attach to this agent. References sources defined in the top-level `rag` section. See [RAG]({{ '/features/rag/' | relative_url }}).                                       |
| `skills`                    | bool/array | ✗     | Enable automatic skill discovery. `true` loads all discovered local skills, `false` disables them. A list can mix skill sources (`local` or `https://…` URLs) and skill names to include — see [Skills]({{ '/features/skills/' | relative_url }}).                                                     |
| `commands`                  | object  | ✗        | Named prompts that can be run with `docker agent run config.yaml /command_name`.                                                                                              |
| `welcome_message`           | string  | ✗        | Message displayed to the user when a session starts. Useful for providing context or instructions.                                                                            |
| `handoffs`                  | array   | ✗        | List of agent names this agent can hand off the conversation to. Enables the `handoff` tool. See [Handoffs Routing]({{ '/concepts/multi-agent/#handoffs-routing' | relative_url }}).                  |
| `hooks`                     | object  | ✗        | Lifecycle hooks for running commands at various points. See [Hooks]({{ '/configuration/hooks/' | relative_url }}).                                                                                   |
| `structured_output`         | object  | ✗        | Constrain agent output to match a JSON schema. See [Structured Output]({{ '/configuration/structured-output/' | relative_url }}).                                                                    |

<div class="callout callout-warning" markdown="1">
<div class="callout-title">⚠️ max_iterations
</div>
  <p>Default is <code>0</code> (unlimited). Always set <code>max_iterations</code> for agents with powerful tools like <code>shell</code> to prevent infinite loops. A value of 20–50 is typical for development agents.</p>

</div>

## Welcome Message

Display a message when users start a session:

```yaml
agents:
  assistant:
    model: openai/gpt-4o
    description: Development assistant
    instruction: You are a helpful coding assistant.
    welcome_message: |
      👋 Welcome! I'm your development assistant.

      I can help you with:
      - Writing and reviewing code
      - Running tests and debugging
      - Explaining concepts

      What would you like to work on?
```

## Deferred Tool Loading

Toolsets support `defer` to load tools on-demand and speed up agent startup. See [Deferred Tool Loading]({{ '/configuration/tools/#deferred-tool-loading' | relative_url }}) for details.

```yaml
agents:
  root:
    model: anthropic/claude-sonnet-4-0
    description: Multi-purpose assistant
    instruction: You have access to many tools.
    toolsets:
      - type: mcp
        ref: docker:github-official
        defer: true
      - type: filesystem
```

## Fallback Configuration

Automatically switch to backup models when the primary fails:

| Property   | Type   | Default | Description                                                |
| ---------- | ------ | ------- | ---------------------------------------------------------- |
| `models`   | array  | `[]`    | Fallback models to try in order                            |
| `retries`  | int    | `2`     | Retries per model for 5xx errors. `-1` to disable.         |
| `cooldown` | string | `1m`    | How long to stick with a fallback after a rate limit (429) |

**Error handling:**

- **Retryable** (same model with backoff): HTTP 5xx, 408, network timeouts
- **Non-retryable** (skip to next model): HTTP 429, 4xx client errors

```yaml
agents:
  root:
    model: anthropic/claude-sonnet-4-0
    fallback:
      models:
        - openai/gpt-4o
        - google/gemini-2.5-flash
      retries: 2
      cooldown: 1m
```

## Named Commands

Define reusable prompt shortcuts:

```yaml
agents:
  root:
    model: openai/gpt-4o
    instruction: You are a system administrator.
    commands:
      df: "Check how much free space I have on my disk"
      logs: "Show me the last 50 lines of system logs"
      greet: "Say hello to ${env.USER}"
      deploy: "Deploy ${env.PROJECT_NAME || 'app'} to ${env.ENV || 'staging'}"
```

```bash
# Run commands from the CLI
$ docker agent run agent.yaml /df
$ docker agent run agent.yaml /greet
$ PROJECT_NAME=myapp ENV=production docker agent run agent.yaml /deploy
```

Commands use JavaScript template literal syntax for environment variable interpolation. Undefined variables expand to empty strings.

## Complete Example

```yaml
models:
  claude:
    provider: anthropic
    model: claude-sonnet-4-0
    max_tokens: 64000

agents:
  root:
    model: claude
    description: Technical lead coordinating development
    instruction: |
      You are a technical lead. Analyze requests and delegate
      to the right specialist. Always review work before responding.
    welcome_message: "👋 I'm your tech lead. How can I help today?"
    sub_agents: [developer, researcher]
    add_date: true
    add_environment_info: true
    fallback:
      models: [openai/gpt-4o]
    toolsets:
      - type: think
    commands:
      review: "Review all recent code changes for issues"
    hooks:
      session_start:
        - type: command
          command: "./scripts/setup.sh"

  developer:
    model: claude
    description: Expert software developer
    instruction: Write clean, tested, production-ready code.
    max_iterations: 30
    toolsets:
      - type: filesystem
      - type: shell
      - type: think
      - type: todo

  researcher:
    model: openai/gpt-4o
    description: Web researcher with memory
    instruction: Search for information and remember findings.
    toolsets:
      - type: mcp
        ref: docker:duckduckgo
      - type: memory
        path: ./research.db
```

---
title: "Anthropic"
description: "Use Claude Sonnet 4, Claude Sonnet 4.5, and other Anthropic models with docker-agent."
permalink: /providers/anthropic/
---

# Anthropic

_Use Claude Sonnet 4, Claude Sonnet 4.5, and other Anthropic models with docker-agent._

## Setup

```bash
# Set your API key
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Configuration

### Inline

```yaml
agents:
  root:
    model: anthropic/claude-sonnet-4-0
```

### Named Model

```yaml
models:
  claude:
    provider: anthropic
    model: claude-sonnet-4-0
    max_tokens: 64000
```

## Available Models

| Model ID            | Description                                         |
| ------------------- | --------------------------------------------------- |
| `claude-opus-4-7`   | Highest-capability Opus model; supports task budget |
| `claude-sonnet-4-5` | Most capable Sonnet, extended thinking (default)    |
| `claude-sonnet-4-0` | Strong coding, balanced performance                 |
| `claude-haiku-4-5`  | Fast and inexpensive, good for tight loops          |

## Thinking Budget

Anthropic uses integer token budgets (1024–32768). Defaults to 8192 with interleaved thinking enabled:

```yaml
models:
  claude-deep:
    provider: anthropic
    model: claude-sonnet-4-5
    thinking_budget: 16384 # must be < max_tokens
```

## Interleaved Thinking

Enabled by default. Allows tool calls during model reasoning for more integrated problem-solving:

```yaml
models:
  claude:
    provider: anthropic
    model: claude-sonnet-4-5
    provider_opts:
      interleaved_thinking: false # disable if needed
```

## Task Budget

`task_budget` caps the **total** number of tokens the model may spend across a
multi-step agentic task — combined thinking, tool calls, and final output. It
is forwarded as
[`output_config.task_budget`](https://platform.claude.com/docs/en/about-claude/models/whats-new-claude-4-7)
and is ideal for letting long-running agents self-regulate effort without
tightening `max_tokens` on every call.

docker-agent automatically attaches the required `task-budgets-2026-03-13`
beta header whenever this field is set. You can configure `task_budget` on
**any** Claude model — docker-agent never gates it by model name. At the time
of writing, only **Claude Opus 4.7** actually honors the field; other Claude
models (Sonnet 4.5, Opus 4.5 / 4.6, etc.) are expected to reject requests
that include it. Check the Anthropic release notes linked above for the
current list of supported models.

```yaml
models:
  opus:
    provider: anthropic
    model: claude-opus-4-7
    task_budget: 128000 # integer shorthand → { type: tokens, total: 128000 }
    thinking_budget: adaptive
```

Object form (forward-compatible with future budget types):

```yaml
  opus:
    provider: anthropic
    model: claude-opus-4-7
    task_budget:
      type: tokens
      total: 128000
```

See the full schema on the [Model Configuration]({{ '/configuration/models/#task-budget' | relative_url }}) page.

## Thinking Display

Controls whether thinking blocks are returned in responses when thinking is enabled. Claude Opus 4.7 hides thinking content by default (`omitted`); earlier Claude 4 models default to `summarized`. Set `thinking_display` in `provider_opts` to override:

```yaml
models:
  claude-opus-4-7:
    provider: anthropic
    model: claude-opus-4-7
    thinking_budget: adaptive
    provider_opts:
      thinking_display: summarized # "summarized", "display", or "omitted"
```

Valid values:

- `summarized`: thinking blocks are returned with summarized thinking text (default for Claude 4 models prior to Opus 4.7).
- `display`: thinking blocks are returned for display (use this to re-enable thinking output on Opus 4.7).
- `omitted`: thinking blocks are returned with an empty thinking field; the signature is still returned for multi-turn continuity (default for Opus 4.7). Useful to reduce time-to-first-text-token when streaming.

Note: `thinking_display` applies to both `thinking_budget` with token counts and adaptive/effort-based budgets. Full thinking tokens are billed regardless of the `thinking_display` value.

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Note
</div>
  <p>Anthropic thinking budget values below 1024 or greater than or equal to <code>max_tokens</code> are ignored (a warning is logged).</p>

</div>

---
title: "Docker Model Runner"
description: "Run AI models locally with Docker — no API keys, no costs, full data privacy."
permalink: /providers/dmr/
---

# Docker Model Runner

_Run AI models locally with Docker — no API keys, no costs, full data privacy._

## Overview

Docker Model Runner (DMR) lets you run open-source AI models directly on your machine. Models run in Docker, so there's no API key needed and no data leaves your computer.

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 No API key needed
</div>
  <p>DMR runs models locally — your data never leaves your machine. Great for development, sensitive data, or offline use.</p>

</div>

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) with the Model Runner feature enabled
- Verify with: `docker model status --json`

## Configuration

### Inline

```yaml
agents:
  root:
    model: dmr/ai/qwen3
```

### Named Model

```yaml
models:
  local:
    provider: dmr
    model: ai/qwen3
    max_tokens: 8192
```

## Available Models

Any model available through Docker Model Runner can be used. Common options:

| Model         | Description                                           |
| ------------- | ----------------------------------------------------- |
| `ai/qwen3`    | Qwen 3 — versatile, good for coding and general tasks |
| `ai/llama3.2` | Llama 3.2 — Meta's open-source model                  |

## Runtime Flags

Pass flags to the underlying inference runtime (e.g., llama.cpp) using `provider_opts.runtime_flags`:

```yaml
models:
  local:
    provider: dmr
    model: ai/qwen3
    max_tokens: 8192
    provider_opts:
      runtime_flags: ["--threads", "8"]
```

Runtime flags also accept a single string:

```yaml
provider_opts:
  runtime_flags: "--threads 8"
```

Use only flags your Model Runner backend allows (see `docker model configure --help` and backend docs). **Do not** put sampling parameters (`temperature`, `top_p`, penalties) in `runtime_flags` — set them on the model (`temperature`, `top_p`, etc.); they are sent **per request** via the OpenAI-compatible chat API.

## Context size

`max_tokens` controls the **maximum output tokens** per chat completion request. To set the engine's **total context window**, use `provider_opts.context_size`:

```yaml
models:
  local:
    provider: dmr
    model: ai/qwen3
    max_tokens: 4096            # max output tokens (per-request)
    provider_opts:
      context_size: 32768       # total context window (sent via _configure)
```

If `context_size` is omitted, Model Runner uses its default. `max_tokens` is **not** used as the context window.

## Thinking / reasoning budget

When using the **llama.cpp** backend, `thinking_budget` is sent as structured `llamacpp.reasoning-budget` on `_configure` (maps to `--reasoning-budget`). String efforts use the same token mapping as other providers; `adaptive` maps to unlimited (`-1`).

When using the **vLLM** backend, `thinking_budget` is sent as `thinking_token_budget` in each chat completion request. Effort levels map to token counts using the same scale as other providers; `adaptive` maps to unlimited (`-1`).

```yaml
models:
  local:
    provider: dmr
    model: ai/qwen3
    thinking_budget: medium   # llama.cpp: reasoning-budget=8192; vLLM: thinking_token_budget=8192
```

On **MLX** and **SGLang** backends, `thinking_budget` is silently ignored — those engines do not currently expose a per-request reasoning token budget knob.

## vLLM-specific configuration

When running a model on the **vLLM** backend, additional engine-level settings can be passed via `provider_opts` and are forwarded to model-runner's `_configure` endpoint:

- `gpu_memory_utilization` — fraction of GPU memory (0.0–1.0) vLLM may use. Values outside this range are rejected.
- `hf_overrides` — map of Hugging Face config overrides applied when vLLM loads the model.

```yaml
models:
  vllm-local:
    provider: dmr
    model: ai/some-model-safetensors
    provider_opts:
      gpu_memory_utilization: 0.9
      hf_overrides:
        max_model_len: 8192
        dtype: bfloat16
```

`hf_overrides` keys (including nested ones) must match `^[a-zA-Z_][a-zA-Z0-9_]*$` — the same rule model-runner enforces server-side to block injection via flags. Invalid keys are rejected at client creation time so you fail fast instead of after a round-trip.

These options are ignored on non-vLLM backends.

## Keeping models resident in memory (`keep_alive`)

By default model-runner unloads idle models after a few minutes. Override the idle timeout via `provider_opts.keep_alive`:

```yaml
models:
  sticky:
    provider: dmr
    model: ai/qwen3
    provider_opts:
      keep_alive: "30m"   # duration string
      # keep_alive: "0"   # unload immediately after each request
      # keep_alive: "-1"  # keep loaded forever
```

Accepted values: any Go duration string (`"30s"`, `"5m"`, `"1h"`, `"2h30m"`), `"0"` (immediate unload), or `"-1"` (never unload). Invalid values are rejected before the configure request is sent.

## Operating mode (`mode`)

Model-runner normally infers the backend mode from the request path. You can pin it explicitly via `provider_opts.mode`:

```yaml
provider_opts:
  mode: embedding   # one of: completion, embedding, reranking, image-generation
```

Most agents don't need this — leave it unset unless you know you need it.

## Raw runtime flags (`raw_runtime_flags`)

`runtime_flags` (a list) is the preferred way to pass flags. If you have a pre-built command-line string you'd rather ship verbatim, use `raw_runtime_flags` instead:

```yaml
provider_opts:
  raw_runtime_flags: "--threads 8 --batch-size 512"
```

Model-runner parses the string with shell-style word splitting. `runtime_flags` and `raw_runtime_flags` are mutually exclusive — setting both is an error.

## Speculative Decoding

Use a smaller draft model to predict tokens ahead for faster inference:

```yaml
models:
  fast-local:
    provider: dmr
    model: ai/qwen3:14B
    max_tokens: 8192
    provider_opts:
      speculative_draft_model: ai/qwen3:0.6B-F16
      speculative_num_tokens: 16
      speculative_acceptance_rate: 0.8
```

## Custom Endpoint

If `base_url` is omitted, docker-agent auto-discovers the DMR endpoint. To set manually:

```yaml
models:
  local:
    provider: dmr
    model: ai/qwen3
    base_url: http://127.0.0.1:12434/engines/llama.cpp/v1
```

## Troubleshooting

- **Plugin not found:** Ensure Docker Model Runner is enabled in Docker Desktop. docker-agent will fall back to the default URL.
- **Endpoint empty:** Verify the Model Runner is running with `docker model status --json`.
- **Performance:** Use `runtime_flags` to tune GPU layers (`--ngl`) and thread count (`--threads`).

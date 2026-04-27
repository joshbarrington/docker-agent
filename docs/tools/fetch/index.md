---
title: "Fetch Tool"
description: "Read content from HTTP/HTTPS URLs."
permalink: /tools/fetch/
---

# Fetch Tool

_Read content from HTTP/HTTPS URLs._

## Overview

The fetch tool lets agents retrieve content from one or more HTTP/HTTPS URLs. It is **read-only** — only `GET` requests are supported. The tool respects `robots.txt`, limits response size (1 MB per URL), and can return content as plain text, Markdown (converted from HTML), or raw HTML.

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ GET only
</div>
  <p>The fetch tool does <strong>not</strong> support <code>POST</code>, <code>PUT</code>, <code>DELETE</code> or other methods, and does not expose request bodies or custom headers. To call REST endpoints with other verbs, use the <a href="{{ '/tools/api/' | relative_url }}">API tool</a> or an <a href="{{ '/configuration/tools/#openapi' | relative_url }}">OpenAPI toolset</a>.</p>

</div>

## Configuration

```yaml
toolsets:
  - type: fetch
```

### Options

| Property  | Type | Default | Description                                                       |
| --------- | ---- | ------- | ----------------------------------------------------------------- |
| `timeout` | int  | `30`    | Default request timeout in seconds (overridable per tool call).   |

### Custom Timeout

```yaml
toolsets:
  - type: fetch
    timeout: 60
```

## Tool Interface

The toolset exposes a single tool, `fetch`, with the following parameters:

| Parameter | Type           | Required | Description                                                                                                 |
| --------- | -------------- | -------- | ----------------------------------------------------------------------------------------------------------- |
| `urls`    | array[string]  | ✓        | One or more HTTP/HTTPS URLs to fetch (all via `GET`).                                                       |
| `format`  | string         | ✓        | Output format: `text`, `markdown`, or `html`. HTML responses are converted to text/markdown when requested. |
| `timeout` | integer        | ✗        | Per-call request timeout in seconds. Overrides the toolset default. Valid range: `1`–`300`.                 |

Responses are capped at **1 MB** per URL. Hosts that disallow the agent's user-agent via `robots.txt` are skipped with a clear error.

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 Fetch vs. API Tool
</div>
  <p>Use <code>fetch</code> when the agent needs to read arbitrary public URLs at runtime. Use the <a href="{{ '/tools/api/' | relative_url }}">API tool</a> to expose specific, structured HTTP endpoints (including non-<code>GET</code> verbs) as named tools.</p>
</div>

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

| Property          | Type           | Default | Description                                                                                                                                                                                                                                                                                                                  |
| ----------------- | -------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `timeout`         | int            | `30`    | Default request timeout in seconds (overridable per tool call).                                                                                                                                                                                                                                                              |
| `allowed_domains` | array[string]  | _none_  | Allow-list of hosts the tool may fetch. When set, every URL whose host is **not** in the list is rejected before any network call is made. Mutually exclusive with `blocked_domains`.                                                                                                                                        |
| `blocked_domains` | array[string]  | _none_  | Deny-list of hosts the tool must not fetch. URLs whose host matches one of these patterns are rejected before any network call (including `robots.txt`) is made. Mutually exclusive with `allowed_domains`.                                                                                                                  |

### Domain matching

Domain patterns in `allowed_domains` and `blocked_domains` use the following rules (case-insensitive):

- **Bare domain** — `example.com` matches the host `example.com` _and_ any subdomain such as `docs.example.com`. It does **not** match unrelated hosts that share a suffix (e.g. `badexample.com`).
- **Leading dot** — `.example.com` matches **only** strict subdomains (`docs.example.com`, `a.b.example.com`), not the apex `example.com`.
- **IP literal** — IP addresses are matched exactly (`169.254.169.254`).
- **Trailing dots** in FQDN-form URLs (`http://example.com./`) are stripped before matching, so they cannot bypass a deny-list entry.

The lists are mutually exclusive: a single fetch toolset may set either `allowed_domains` or `blocked_domains`, but not both.

When a list is configured, every redirect target is re-checked against the same list. A request to an allowed origin that redirects to a forbidden host is rejected before any data is read from the redirect.

<div class="callout callout-warning" markdown="1">
<div class="callout-title">⚠️ Limitations
</div>
  <p>Matching is purely string-based on the URL host. It does <strong>not</strong> perform DNS resolution and does <strong>not</strong> normalise alternative IP encodings (decimal <code>2852039166</code>, hex <code>0xa9.0xfe.0xa9.0xfe</code>, octal, IPv4-mapped IPv6, etc.). If you need to deny access to a specific IP, also list its alternative encodings, or block at the network layer.</p>
</div>

### Custom Timeout

```yaml
toolsets:
  - type: fetch
    timeout: 60
```

### Restrict to specific domains

```yaml
toolsets:
  - type: fetch
    allowed_domains:
      - docker.com          # docker.com and *.docker.com
      - github.com          # github.com and *.github.com
      - .githubusercontent.com  # only subdomains, e.g. raw.githubusercontent.com
```

### Block sensitive hosts

```yaml
toolsets:
  - type: fetch
    blocked_domains:
      - 169.254.169.254       # cloud metadata endpoint
      - internal.example.com  # internal corporate hostnames
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

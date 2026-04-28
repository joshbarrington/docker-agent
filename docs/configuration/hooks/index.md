---
title: "Hooks"
description: "Run shell commands at various points during agent execution for deterministic control over behavior."
permalink: /configuration/hooks/
---

# Hooks

_Run shell commands at various points during agent execution for deterministic control over behavior._

## Overview

Hooks allow you to execute shell commands or scripts at key points in an agent's lifecycle. They provide deterministic control that works alongside the LLM's behavior, enabling validation, logging, environment setup, and more.

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Use Cases
</div>

- Validate or transform tool inputs before execution
- Log all tool calls to an audit file
- Block dangerous operations based on custom rules
- Validate, redact, or enrich user prompts before they reach the model
- Programmatically approve or deny tool calls without prompting the user
- Steer or veto context-window compaction
- Audit sub-agent handoffs in multi-agent setups
- Set up the environment when a session starts
- Clean up resources when a session ends
- Log or validate model responses before returning to the user
- Send external notifications on agent errors or warnings

</div>

## Hook Types

There are seven hook event types:

| Event               | When it fires                                                       | Can block? |
| ------------------- | ------------------------------------------------------------------- | ---------- |
| `pre_tool_use`      | Before a tool call executes                                         | Yes        |
| `post_tool_use`     | After a tool completes — fires for both success and failure         | Yes        |
| `permission_request`| Just before the runtime would prompt the user to approve a tool     | Yes        |
| `session_start`     | When a session begins or resumes                                    | No         |
| `user_prompt_submit`| Once per user message, after submission and before the model runs   | Yes        |
| `session_end`       | When a session terminates                                           | No         |
| `pre_compact`       | Just before the runtime compacts the session transcript             | Yes        |
| `subagent_stop`     | When a sub-agent (transferred task / background) finishes           | No         |
| `on_user_input`     | When the agent is waiting for user input                            | No         |
| `stop`              | When the model finishes responding                                  | No         |
| `notification`      | When the agent emits a notification (error or warning)              | No         |

## Configuration

```yaml
agents:
  root:
    model: openai/gpt-4o
    description: An agent with hooks
    instruction: You are a helpful assistant.
    hooks:
      # Run before specific tools
      pre_tool_use:
        - matcher: "shell|edit_file"
          hooks:
            - type: command
              command: "./scripts/validate-command.sh"
              timeout: 30

      # Run after all tool calls
      post_tool_use:
        - matcher: "*"
          hooks:
            - type: command
              command: "./scripts/log-tool-call.sh"

      # Run when session starts
      session_start:
        - type: command
          command: "./scripts/setup-env.sh"

      # Run when session ends
      session_end:
        - type: command
          command: "./scripts/cleanup.sh"

      # Run when agent is waiting for user input
      on_user_input:
        - type: command
          command: "./scripts/notify.sh"

      # Run when the model finishes responding
      stop:
        - type: command
          command: "./scripts/log-response.sh"

      # Run on agent errors and warnings
      notification:
        - type: command
          command: "./scripts/alert.sh"
```

## Built-in Hooks

In addition to shell `command` hooks, docker-agent ships a small library of **built-in hooks** — in-process Go functions that run without spawning a subprocess. They're invoked with `type: builtin`, where `command` is the builtin's registered name and `args` are passed through as the builtin's parameters.

```yaml
hooks:
  turn_start:
    - type: builtin
      command: add_date
    - type: builtin
      command: add_prompt_files
      args:
        - GUIDELINES.md
        - PROJECT.md
  session_start:
    - type: builtin
      command: add_environment_info
  before_llm_call:
    - type: builtin
      command: max_iterations
      args: ["50"]
```

Built-ins are typically zero-config and faster than equivalent shell hooks because they don't fork a process. They cover the common "inject context into every turn / session" patterns out of the box.

### Available built-ins

| Builtin                 | Event             | Args                   | What it does                                                                                                          |
| ----------------------- | ----------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `add_date`              | `turn_start`      | _none_                 | Prepends `Today's date: YYYY-MM-DD` so the model always knows the current date.                                       |
| `add_environment_info`  | `session_start`   | _none_                 | Adds the working directory, git-repo status, OS, and CPU architecture.                                                |
| `add_prompt_files`      | `turn_start`      | `[file1, file2, ...]`  | Reads each named file from the workdir hierarchy (walking up) and the home directory, and appends their contents.     |
| `add_git_status`        | `turn_start`      | _none_                 | Adds the output of `git status --short --branch` (no-op outside a git repo or when git isn't installed).              |
| `add_git_diff`          | `turn_start`      | _none_, or `["full"]`  | Adds `git diff --stat` by default. Pass `args: ["full"]` to emit the full unified diff. Output is capped to 4 KB.     |
| `add_directory_listing` | `session_start`   | _none_                 | Adds an alphabetical listing of the cwd's top-level entries (skips dot-files, capped at 100 with a "... and N more"). |
| `add_user_info`         | `session_start`   | _none_                 | Adds the current OS user (username and full name) and the hostname.                                                   |
| `add_recent_commits`    | `session_start`   | _none_, or `["<N>"]`   | Adds `git log --oneline -n N`. `N` defaults to 10; pass a positive integer to override.                               |
| `max_iterations`        | `before_llm_call` | `["<N>"]` (required)   | Hard-stops the agent after `N` model calls. State is per-session and reset at `session_end`.                          |

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Per-turn vs. per-session
</div>
  <p><code>turn_start</code> built-ins recompute every turn and contribute <strong>transient</strong> context that is <em>not</em> persisted to the session — perfect for fast-moving signals like the date or current git state. <code>session_start</code> built-ins run once per session and their context <strong>persists</strong> across turns and resumes — pick this for stable context like the OS user or the initial directory listing.</p>
</div>

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Auto-injected built-ins
</div>
  <p>The agent flags <code>add_date: true</code>, <code>add_environment_info: true</code>, and <code>add_prompt_files: [...]</code> are shorthands that auto-register the matching built-in hook. You don't need to repeat them under <code>hooks:</code> — set the flag <em>or</em> the hook entry, not both.</p>
</div>

<div class="callout callout-warning" markdown="1">
<div class="callout-title">⚠️ Two flavors of <code>max_iterations</code>
</div>
  <p>The <code>max_iterations</code> agent field has its own UX (it pauses and asks the user to resume past the limit). The <code>max_iterations</code> built-in hook is a <strong>hard stop with no resume</strong> — when its counter trips, the agent terminates with a block decision. Use the agent field for interactive sessions and the built-in hook to enforce non-negotiable caps in unattended runs.</p>
</div>

## Matcher Patterns

The `matcher` field uses regex patterns to match tool names:

| Pattern            | Matches                       |
| ------------------ | ----------------------------- |
| `*`                | All tools                     |
| `shell`            | Only the `shell` tool         |
| `shell\|edit_file` | Either `shell` or `edit_file` |
| `mcp:.*`           | All MCP tools (regex)         |

## Hook Input

Hooks receive JSON input via stdin with context about the event:

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "hook_event_name": "pre_tool_use",
  "tool_name": "shell",
  "tool_use_id": "call_xyz",
  "tool_input": {
    "cmd": "rm -rf /tmp/cache",
    "cwd": "."
  }
}
```

### Input Fields by Event Type

| Field                  | pre_tool_use | post_tool_use | permission_request | session_start | user_prompt_submit | session_end | pre_compact | subagent_stop | on_user_input | stop | notification |
| ---------------------- | ------------ | ------------- | ------------------ | ------------- | ------------------ | ----------- | ----------- | ------------- | ------------- | ---- | ------------ |
| `session_id`           | ✓            | ✓             | ✓                  | ✓             | ✓                  | ✓           | ✓           | ✓             | ✓             | ✓    | ✓            |
| `cwd`                  | ✓            | ✓             | ✓                  | ✓             | ✓                  | ✓           | ✓           | ✓             | ✓             | ✓    | ✓            |
| `hook_event_name`      | ✓            | ✓             | ✓                  | ✓             | ✓                  | ✓           | ✓           | ✓             | ✓             | ✓    | ✓            |
| `tool_name`            | ✓            | ✓             | ✓                  |               |                    |             |             |               |               |      |              |
| `tool_use_id`          | ✓            | ✓             | ✓                  |               |                    |             |             |               |               |      |              |
| `tool_input`           | ✓            | ✓             | ✓                  |               |                    |             |             |               |               |      |              |
| `tool_response`        |              | ✓             |                    |               |                    |             |             |               |               |      |              |
| `source`               |              |               |                    | ✓             |                    |             | ✓           |               |               |      |              |
| `reason`               |              |               |                    |               |                    | ✓           |             |               |               |      |              |
| `prompt`               |              |               |                    |               | ✓                  |             |             |               |               |      |              |
| `agent_name`           |              |               |                    |               |                    |             |             | ✓             |               |      |              |
| `parent_session_id`    |              |               |                    |               |                    |             |             | ✓             |               |      |              |
| `stop_response`        |              |               |                    |               |                    |             |             | ✓             |               | ✓    |              |
| `notification_level`   |              |               |                    |               |                    |             |             |               |               |      | ✓            |
| `notification_message` |              |               |                    |               |                    |             |             |               |               |      | ✓            |

The `source` field for `session_start` can be: `startup`, `resume`, `clear`, or `compact`.
The `source` field for `pre_compact` can be: `manual` (user-initiated `/compact`), `auto` (proactive threshold), `overflow` (context-overflow recovery), or `tool_overflow` (proactive recovery after tool results pushed past the threshold).

The `reason` field for `session_end` can be: `clear`, `logout`, `prompt_input_exit`, or `other`.

The `prompt` field for `user_prompt_submit` is the text the user just submitted. Sub-sessions (transferred tasks, background agents, skills) do **not** fire this event because their kick-off message is synthesised by the runtime, not authored by the user.

The `agent_name` field for `subagent_stop` is the name of the sub-agent that just finished; `parent_session_id` is the session that spawned it.

The `stop_response` field contains the model's final text response (for both `stop` and `subagent_stop`).

The `notification_level` field can be: `error` or `warning`.

## Hook Output

Hooks communicate back via JSON output to stdout:

```json
{
  "continue": true,
  "stop_reason": "Optional message when continue=false",
  "suppress_output": false,
  "system_message": "Warning message to show user",
  "decision": "allow",
  "reason": "Explanation for the decision",
  "hook_specific_output": {
    "hook_event_name": "pre_tool_use",
    "permission_decision": "allow",
    "permission_decision_reason": "Command is safe",
    "updated_input": { "cmd": "modified command" }
  }
}
```

### Output Fields

| Field             | Type    | Description                                     |
| ----------------- | ------- | ----------------------------------------------- |
| `continue`        | boolean | Whether to continue execution (default: `true`) |
| `stop_reason`     | string  | Message to show when `continue=false`           |
| `suppress_output` | boolean | Hide stdout from transcript                     |
| `system_message`  | string  | Warning message to display to user              |
| `decision`        | string  | For blocking: `block` to prevent operation      |
| `reason`          | string  | Explanation for the decision                    |

### Pre-Tool-Use Specific Output

The `hook_specific_output` for `pre_tool_use` supports:

| Field                        | Type   | Description                             |
| ---------------------------- | ------ | --------------------------------------- |
| `permission_decision`        | string | `allow`, `deny`, or `ask`               |
| `permission_decision_reason` | string | Explanation for the decision            |
| `updated_input`              | object | Modified tool input (replaces original) |

### Plain Text Output

For `session_start`, `user_prompt_submit`, `post_tool_use`, `pre_compact`, and `stop` hooks, plain text written to stdout (i.e., output that is not valid JSON) is captured as additional context for the agent. For `pre_compact` it is appended to the compaction prompt; for the others it is spliced into the conversation as a (transient or persisted) system message depending on the event.

## Exit Codes

Hook exit codes have special meaning:

| Exit Code | Meaning                                |
| --------- | -------------------------------------- |
| `0`       | Success — continue normally            |
| `2`       | Blocking error — stop the operation    |
| Other     | Error — logged but execution continues |

## Per-hook options

Hooks have a default timeout of 60 seconds. You can also give hooks a name, add environment variables, choose a working directory, and control how non-security hook failures behave:

```yaml
hooks:
  post_tool_use:
    - matcher: "shell"
      hooks:
        - name: "summarize shell output"
          type: command
          command: "./summarize.sh"
          timeout: 120 # 2 minutes
          working_dir: ./hooks
          env:
            PROFILE: dev
          on_error: warn # warn | ignore | block
```

`pre_tool_use` is fail-closed for safety: a failed pre-tool hook blocks the tool call regardless of `on_error`.

<div class="callout callout-warning" markdown="1">
<div class="callout-title">⚠️ Performance
</div>
  <p>Hooks run synchronously and can slow down agent execution. Keep hook scripts fast and efficient. Consider using <code>suppress_output: true</code> for logging hooks to reduce noise.</p>

</div>

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Session End and Cancellation
</div>
  <p><code>session_end</code> hooks are designed to run even when the session is interrupted (e.g., Ctrl+C). They are still subject to their configured timeout.</p>

</div>

## Examples

### Validation Script

A simple pre-tool-use hook that blocks dangerous shell commands:

```bash
#!/bin/bash
# scripts/validate-command.sh

# Read JSON input from stdin
INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name')
CMD=$(echo "$INPUT" | jq -r '.tool_input.cmd // empty')

# Block dangerous commands
if [[ "$TOOL_NAME" == "shell" ]]; then
  if [[ "$CMD" =~ ^sudo ]] || [[ "$CMD" =~ rm.*-rf ]]; then
    echo '{"decision": "block", "reason": "Dangerous command blocked by policy"}'
    exit 2
  fi
fi

# Allow everything else
echo '{"decision": "allow"}'
exit 0
```

### Audit Logging

A post-tool-use hook that logs all tool calls:

```bash
#!/bin/bash
# scripts/log-tool-call.sh

INPUT=$(cat)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name')
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')

# Append to audit log
echo "$TIMESTAMP | $SESSION_ID | $TOOL_NAME" >> ./audit.log

# Don't block execution
echo '{"continue": true}'
exit 0
```

### Session Lifecycle

Session start and end hooks for environment setup and cleanup:

```yaml
hooks:
  session_start:
    - type: command
      timeout: 10
      command: |
        INPUT=$(cat)
        SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"')
        echo "Session $SESSION_ID started at $(date)" >> /tmp/agent-session.log
        echo '{"hook_specific_output":{"additional_context":"Session initialized."}}'

  session_end:
    - type: command
      timeout: 10
      command: |
        INPUT=$(cat)
        SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"')
        REASON=$(echo "$INPUT" | jq -r '.reason // "unknown"')
        echo "Session $SESSION_ID ended ($REASON) at $(date)" >> /tmp/agent-session.log
```

### Response Logging with Stop Hook

Log every model response for analytics or compliance:

```yaml
hooks:
  stop:
    - type: command
      timeout: 10
      command: |
        INPUT=$(cat)
        SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"')
        RESPONSE_LENGTH=$(echo "$INPUT" | jq -r '.stop_response // ""' | wc -c | tr -d ' ')
        echo "[$(date)] Session $SESSION_ID - Response: $RESPONSE_LENGTH chars" >> /tmp/agent-responses.log
```

The `stop` hook is useful for:

- **Response quality checks** — validate that responses meet criteria before returning
- **Analytics** — track response lengths, patterns, or content
- **Compliance logging** — record all agent outputs for audit

### Error Notifications

Send alerts when the agent encounters errors:

```yaml
hooks:
  notification:
    - type: command
      timeout: 10
      command: |
        INPUT=$(cat)
        LEVEL=$(echo "$INPUT" | jq -r '.notification_level // "unknown"')
        MESSAGE=$(echo "$INPUT" | jq -r '.notification_message // "no message"')
        echo "[$(date)] [$LEVEL] $MESSAGE" >> /tmp/agent-notifications.log
```

The `notification` hook fires when:

- The model returns an error (all models failed)
- A degenerate tool call loop is detected
- The maximum iteration limit is reached

### Pre-Compact: steer the summary

`pre_compact` fires just before the runtime compacts the session transcript. Its `source` field tells you why compaction was triggered:

- `manual` — the user invoked `/compact`
- `auto` — proactive compaction at the configured threshold
- `overflow` — emergency compaction after a context-overflow error
- `tool_overflow` — proactive compaction triggered by tool results pushing the estimated context past the threshold

Return `additional_context` (or plain stdout) to append guidance to the compaction prompt without modifying the agent's instruction. Block the event (`decision: block` / exit code 2) to cancel compaction — useful when you want to handle truncation yourself.

### User-Prompt-Submit: gate or enrich every user message

`user_prompt_submit` fires once per user message, after the prompt is recorded in the session and before the first model call. The submitted text is in `prompt`. Use it to:

- block prompts that violate policy (`decision: block` / exit code 2),
- inject per-prompt context (`additional_context` is spliced as a transient system message for that turn),
- audit user prompts to a log.

It does **not** fire for sub-sessions (transferred tasks, background agents, skill sub-sessions) because their kick-off message is synthesised by the runtime.

### Subagent-Stop: observe handoff completions

`subagent_stop` fires whenever a sub-agent finishes — `transfer_task` returns, a background agent completes, or a skill sub-session ends. It runs against the *parent* agent's hooks executor, so handlers configured on the orchestrator see every child completion in one place. The sub-agent's name is in `agent_name`, the parent's session ID in `parent_session_id`, and the child's final assistant message in `stop_response`.

### Permission-Request: programmatic tool approval

`permission_request` fires just before the runtime would prompt the user to approve a tool call (i.e. when neither `--yolo` nor a permissions rule short-circuited the decision and the tool is not read-only). Use the same `hook_specific_output.permission_decision` shape as `pre_tool_use` to auto-approve or auto-deny the call:

```yaml
hooks:
  permission_request:
    - matcher: "shell"
      hooks:
        - type: command
          command: |
            INPUT=$(cat)
            CMD=$(echo "$INPUT" | jq -r '.tool_input.cmd // ""')
            if echo "$CMD" | grep -qE '^(ls|pwd|cat) '; then
              echo '{"hook_specific_output":{"permission_decision":"allow","permission_decision_reason":"safe read-only command"}}'
            fi
```

Return nothing to fall through to the usual interactive confirmation.

### LLM as a Judge (Auto-Approving Tool Calls)

The `model` hook type asks an LLM and translates its reply into the
hook's native output — no Go code, no shell glue, no JSON parsing on
your side. Combined with the well-known `pre_tool_use_decision`
schema it gives you a fully-configurable LLM judge that decides
`allow` / `ask` / `deny` per tool call.

```yaml
hooks:
  pre_tool_use:
    - matcher: "shell|edit_file|mcp:.*"
      hooks:
        - type: model
          model: openai/gpt-4o-mini
          timeout: 15
          schema: pre_tool_use_decision
          prompt: |
            You are a security judge for an autonomous agent.
            Decide whether this tool call is safe to auto-approve.

            Tool: {{ .ToolName }}
            Args: {{ .ToolInput | toJSON }}

            Project rules:
            - Reads under the working directory are safe.
            - Writes to ~/.ssh / ~/.aws / ~/.docker are deny.
```

| Field    | Required          | Description                                                                                                                                                                |
| -------- | ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `model`  | yes               | Model spec (`provider/model`, e.g. `openai/gpt-4o-mini`). The judge model — small/cheap is recommended.                                                                    |
| `prompt` | yes               | Go [`text/template`](https://pkg.go.dev/text/template) body. Sees the hook [Input](#hook-input) as data, plus the `toJSON` and `truncate <n>` helpers.                     |
| `schema` | no                | Well-known response interpretation. `pre_tool_use_decision` produces a `permission_decision` verdict; omit for free-form text injected as `additional_context`.            |
| `timeout`| no (default 60s)  | Per-call timeout. **Timeouts fail closed (deny) for `pre_tool_use`** regardless of any other setting. Match it to your judge model's typical latency plus a small buffer. |

The `pre_tool_use_decision` schema constrains the judge to reply with
strict `{decision, reason}` JSON. Providers that honor structured
output (OpenAI, ...) are asked to emit that shape directly; on
providers that ignore it the framework still parses tolerant
JSON-in-text. Anything unparseable propagates as a hook error and the
executor falls closed (deny) on `pre_tool_use`.

Pair it with deterministic `permissions:` rules so destructive calls
(e.g. `sudo`, `rm -rf`) are blocked even if the judge is misled, and
obvious read-only calls bypass the LLM entirely. See
[`examples/llm_judge.yaml`](https://github.com/docker/docker-agent/blob/main/examples/llm_judge.yaml)
for a complete configuration.

**Security considerations**:

- **Sensitive data**: Tool arguments (including file paths, command
  arguments, and any other parameters) are sent to the judge LLM. Avoid
  using the judge on tools that handle secrets, or ensure your judge
  model is self-hosted.
- **Defense in depth**: The judge should not be your only security
  layer. Use deterministic `permissions:` rules to block obviously
  dangerous operations (e.g., `sudo`, `rm -rf`) before the judge sees
  them, as shown in the example configuration.

</div>

## CLI Flags

You can add hooks from the command line without modifying the agent's YAML file. This is useful for one-off debugging, audit logging, or layering hooks onto an existing agent.

| Flag                    | Description                             |
| ----------------------- | --------------------------------------- |
| `--hook-pre-tool-use`   | Run a command before every tool call    |
| `--hook-post-tool-use`  | Run a command after every tool call     |
| `--hook-session-start`  | Run a command when a session starts     |
| `--hook-session-end`    | Run a command when a session ends       |
| `--hook-on-user-input`  | Run a command when waiting for input    |

All flags are repeatable — pass multiple to register multiple hooks.

```bash
# Add a session-start hook
$ docker agent run agent.yaml --hook-session-start "./scripts/setup-env.sh"

# Combine multiple hooks
$ docker agent run agent.yaml \
  --hook-pre-tool-use "./scripts/validate.sh" \
  --hook-post-tool-use "./scripts/log.sh"

# Add hooks to an agent from a registry
$ docker agent run agentcatalog/coder \
  --hook-pre-tool-use "./audit.sh"
```

<div class="callout callout-info" markdown="1">
<div class="callout-title">ℹ️ Merging behavior
</div>
  <p>CLI hooks are <strong>appended</strong> to any hooks already defined in the agent's YAML config. They don't replace existing hooks. Pre/post-tool-use hooks added via CLI match all tools (equivalent to <code>matcher: "*"</code>).</p>

</div>

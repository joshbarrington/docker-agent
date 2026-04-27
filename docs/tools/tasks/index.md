---
title: "Tasks Tool"
description: "Persistent task database shared across sessions."
permalink: /tools/tasks/
---

# Tasks Tool

_Persistent task database shared across sessions._

## Overview

The tasks tool provides a persistent task database that survives across agent sessions. Unlike the [Todo tool]({{ '/tools/todo/' | relative_url }}), which maintains an in-memory task list for the current session only, the tasks tool stores tasks in a SQLite database so they can be accessed and updated across multiple sessions.

## Configuration

```yaml
toolsets:
  - type: tasks
    path: ~/.cagent/tasks.db  # Optional: custom database path
```

### Options

| Property | Type   | Default              | Description                    |
| -------- | ------ | -------------------- | ------------------------------ |
| `path`   | string | `~/.cagent/tasks.db` | Path to the SQLite task database |

## Available Tools

The tasks toolset exposes these tools:

| Tool Name      | Description                                      |
| -------------- | ------------------------------------------------ |
| `add_task`     | Add a new task to the database                   |
| `list_tasks`   | List all tasks, optionally filtered by status    |
| `update_task`  | Update a task's title, description, or status    |
| `delete_task`  | Remove a task from the database                  |

## Example

```yaml
agents:
  root:
    model: openai/gpt-4o
    toolsets:
      - type: tasks
        path: ./project-tasks.db
```

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 Tasks vs. Todo
</div>
  <p>Use the <strong>tasks</strong> tool when you need persistence across sessions (e.g., long-running projects, recurring work). Use the <a href="{{ '/tools/todo/' | relative_url }}">todo tool</a> for ephemeral, session-scoped task lists.</p>

</div>

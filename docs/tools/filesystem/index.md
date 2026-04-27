---
title: "Filesystem Tool"
description: "Read, write, list, search, and navigate files and directories."
permalink: /tools/filesystem/
---

# Filesystem Tool

_Read, write, list, search, and navigate files and directories._

## Overview

The filesystem tool gives agents the ability to explore codebases, read and edit files, create new files, search across files, and navigate directory structures. Paths are resolved relative to the working directory, though agents can also use absolute paths.

## Available Tools

| Tool                   | Description                                                               |
| ---------------------- | ------------------------------------------------------------------------- |
| `read_file`            | Read the complete contents of a file                                      |
| `read_multiple_files`  | Read several files in one call (more efficient than multiple `read_file`) |
| `write_file`           | Create or overwrite a file with new content                               |
| `edit_file`            | Make line-based edits (find-and-replace) in an existing file              |
| `list_directory`       | List files and directories at a given path                                |
| `directory_tree`       | Recursive tree view of a directory                                        |
| `create_directory`     | Create a new directory (creates parent directories as needed)             |
| `remove_directory`     | Remove an empty directory                                                 |
| `search_files_content` | Search for text or regex patterns across files                            |

## Configuration

```yaml
toolsets:
  - type: filesystem
```

### Options

| Property | Type | Default | Description |
| --- | --- | --- | --- |
| `ignore_vcs` | boolean | `false` | When `true`, ignores `.gitignore` patterns and includes all files |
| `post_edit` | array | `[]` | Commands to run after editing files matching a path pattern |
| `post_edit[].path` | string | — | Glob pattern for files (e.g., `*.go`, `src/**/*.ts`) |
| `post_edit[].cmd` | string | — | Command to run (use `${file}` for the edited file path) |

### Post-Edit Hooks

Automatically run formatting or other commands after file edits:

```yaml
toolsets:
  - type: filesystem
    ignore_vcs: false
    post_edit:
      - path: "*.go"
        cmd: "gofmt -w ${file}"
      - path: "*.ts"
        cmd: "prettier --write ${file}"
```

<div class="callout callout-tip" markdown="1">
<div class="callout-title">💡 Tip
</div>
  <p>The filesystem tool resolves paths relative to the working directory. Agents can also use absolute paths.</p>
</div>

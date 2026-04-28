// Package tool builds the TUI view for a tool call message.
//
// A small lookup table (builders) maps each tool's name to a constructor.
// Lookup order is: exact tool name, then "category:<category>", then a
// defaulttool fallback.
package tool

import (
	"github.com/docker/docker-agent/pkg/tools/builtin"
	"github.com/docker/docker-agent/pkg/tui/components/tool/api"
	"github.com/docker/docker-agent/pkg/tui/components/tool/defaulttool"
	"github.com/docker/docker-agent/pkg/tui/components/tool/directorytree"
	"github.com/docker/docker-agent/pkg/tui/components/tool/editfile"
	"github.com/docker/docker-agent/pkg/tui/components/tool/handoff"
	"github.com/docker/docker-agent/pkg/tui/components/tool/listdirectory"
	"github.com/docker/docker-agent/pkg/tui/components/tool/readfile"
	"github.com/docker/docker-agent/pkg/tui/components/tool/readmultiplefiles"
	"github.com/docker/docker-agent/pkg/tui/components/tool/searchfilescontent"
	"github.com/docker/docker-agent/pkg/tui/components/tool/shell"
	"github.com/docker/docker-agent/pkg/tui/components/tool/todotool"
	"github.com/docker/docker-agent/pkg/tui/components/tool/transfertask"
	"github.com/docker/docker-agent/pkg/tui/components/tool/userprompt"
	"github.com/docker/docker-agent/pkg/tui/components/tool/writefile"
	"github.com/docker/docker-agent/pkg/tui/core/layout"
	"github.com/docker/docker-agent/pkg/tui/service"
	"github.com/docker/docker-agent/pkg/tui/types"
)

// builder constructs the layout.Model for a tool message.
type builder func(msg *types.Message, sessionState service.SessionStateReader) layout.Model

// builders maps a tool name (or a "category:<name>" key) to its renderer.
// Tools sharing the same visual representation point at the same builder.
var builders = map[string]builder{
	builtin.ToolNameTransferTask:       transfertask.New,
	builtin.ToolNameHandoff:            handoff.New,
	builtin.ToolNameEditFile:           editfile.New,
	builtin.ToolNameWriteFile:          writefile.New,
	builtin.ToolNameReadFile:           readfile.New,
	builtin.ToolNameReadMultipleFiles:  readmultiplefiles.New,
	builtin.ToolNameListDirectory:      listdirectory.New,
	builtin.ToolNameDirectoryTree:      directorytree.New,
	builtin.ToolNameSearchFilesContent: searchfilescontent.New,
	builtin.ToolNameShell:              shell.New,
	builtin.ToolNameUserPrompt:         userprompt.New,
	builtin.ToolNameFetch:              api.New,
	"category:api":                     api.New,
	builtin.ToolNameCreateTodo:         todotool.New,
	builtin.ToolNameCreateTodos:        todotool.New,
	builtin.ToolNameUpdateTodos:        todotool.New,
	builtin.ToolNameListTodos:          todotool.New,
}

// New returns the appropriate tool view for the given message.
// Lookup order: exact tool name, then "category:<category>", then default.
func New(msg *types.Message, sessionState service.SessionStateReader) layout.Model {
	if b, ok := builders[msg.ToolCall.Function.Name]; ok {
		return b(msg, sessionState)
	}
	if cat := msg.ToolDefinition.Category; cat != "" {
		if b, ok := builders["category:"+cat]; ok {
			return b(msg, sessionState)
		}
	}
	return defaulttool.New(msg, sessionState)
}

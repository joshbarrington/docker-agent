package builtins

import (
	"context"
	"log/slog"
	"strings"

	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
)

// AddPromptFiles is the registered name of the add_prompt_files builtin.
const AddPromptFiles = "add_prompt_files"

// addPromptFiles reads each filename in args (relative to Input.Cwd) and
// joins their contents into turn_start additional context. Missing files
// are logged and skipped; surviving files still contribute.
func addPromptFiles(_ context.Context, in *hooks.Input, args []string) (*hooks.Output, error) {
	if in == nil || in.Cwd == "" || len(args) == 0 {
		return nil, nil
	}
	var parts []string
	for _, name := range args {
		additional, err := session.ReadPromptFiles(in.Cwd, name)
		if err != nil {
			slog.Warn("reading prompt file", "file", name, "error", err)
			continue
		}
		parts = append(parts, additional...)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return hooks.NewAdditionalContextOutput(hooks.EventTurnStart, strings.Join(parts, "\n\n")), nil
}

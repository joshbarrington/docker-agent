// Package builtins contains the stock in-process hook implementations
// shipped with docker-agent: add_date, add_environment_info, and
// add_prompt_files.
//
// They can be referenced explicitly from a hook YAML entry using
// `{type: builtin, command: "<name>"}`. The runtime also auto-injects
// them when the corresponding agent flags (AddDate, AddEnvironmentInfo,
// AddPromptFiles) are set.
//
// AddDate and AddPromptFiles target turn_start so they recompute every
// turn. AddEnvironmentInfo targets session_start because cwd / OS / arch
// don't change during a session.
package builtins

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
)

// Builtin hook names. They match the `command` field of a
// `{type: builtin}` hook entry in YAML.
const (
	AddDate            = "add_date"
	AddEnvironmentInfo = "add_environment_info"
	AddPromptFiles     = "add_prompt_files"
)

// Register installs the stock builtin hooks on r.
func Register(r *hooks.Registry) error {
	for name, fn := range map[string]hooks.BuiltinFunc{
		AddDate:            addDate,
		AddEnvironmentInfo: addEnvironmentInfo,
		AddPromptFiles:     addPromptFiles,
	} {
		if err := r.RegisterBuiltin(name, fn); err != nil {
			return fmt.Errorf("register %q builtin: %w", name, err)
		}
	}
	return nil
}

// turnStartContext wraps additional context as a turn_start output.
func turnStartContext(content string) *hooks.Output {
	return &hooks.Output{
		HookSpecificOutput: &hooks.HookSpecificOutput{
			HookEventName:     hooks.EventTurnStart,
			AdditionalContext: content,
		},
	}
}

// addDate emits today's date as turn_start additional context.
func addDate(_ context.Context, _ *hooks.Input, _ []string) (*hooks.Output, error) {
	return turnStartContext("Today's date: " + time.Now().Format("2006-01-02")), nil
}

// addEnvironmentInfo emits cwd / git / OS / arch info as session_start
// additional context. No-op when Cwd is empty.
func addEnvironmentInfo(_ context.Context, in *hooks.Input, _ []string) (*hooks.Output, error) {
	if in == nil || in.Cwd == "" {
		return nil, nil
	}
	return &hooks.Output{
		HookSpecificOutput: &hooks.HookSpecificOutput{
			HookEventName:     hooks.EventSessionStart,
			AdditionalContext: session.GetEnvironmentInfo(in.Cwd),
		},
	}, nil
}

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
	return turnStartContext(strings.Join(parts, "\n\n")), nil
}

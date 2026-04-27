package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker-agent/pkg/hooks"
	"github.com/docker/docker-agent/pkg/session"
)

// Builtin hook names.
//
// They can also be referenced explicitly from a hook YAML entry using
// `{type: builtin, command: "<name>"}` once we expose builtins in the
// schema.
//
// Behavioral note: AddDate and AddPromptFiles are registered against
// turn_start so they recompute on every model call (matching the
// original per-turn semantics of session.buildContextSpecificSystemMessages).
// AddEnvironmentInfo is registered against session_start because working
// directory, OS, and arch don't change during a session; snapshotting
// once and persisting via Result.AdditionalContext is cheaper than
// re-computing each turn.
const (
	BuiltinAddDate            = "add_date"
	BuiltinAddEnvironmentInfo = "add_environment_info"
	BuiltinAddPromptFiles     = "add_prompt_files"
)

// registerBuiltinHooks installs the runtime-owned builtin hooks on r.
// It is called once from [NewLocalRuntime] so the builtins are available
// to every executor the runtime constructs, without polluting any
// process-wide state. The only failure modes are programmer errors
// (empty name or nil function), which surface as a constructor error.
func registerBuiltinHooks(r *hooks.Registry) error {
	pairs := []struct {
		name string
		fn   hooks.BuiltinFunc
	}{
		{BuiltinAddDate, addDateBuiltin},
		{BuiltinAddEnvironmentInfo, addEnvironmentInfoBuiltin},
		{BuiltinAddPromptFiles, addPromptFilesBuiltin},
	}
	for _, p := range pairs {
		if err := r.RegisterBuiltin(p.name, p.fn); err != nil {
			return fmt.Errorf("register %q builtin: %w", p.name, err)
		}
	}
	return nil
}

// addDateBuiltin returns the current date as additional context. It is
// equivalent to the previous inline `Today's date: ...` system message
// that lived in pkg/session/session.go, lifted into the hook system.
//
// Registered against EventTurnStart so the date is recomputed every
// turn rather than snapshotted at session start, matching the original
// per-turn semantics.
func addDateBuiltin(_ context.Context, _ *hooks.Input, _ []string) (*hooks.Output, error) {
	return &hooks.Output{
		HookSpecificOutput: &hooks.HookSpecificOutput{
			HookEventName:     hooks.EventTurnStart,
			AdditionalContext: "Today's date: " + time.Now().Format("2006-01-02"),
		},
	}, nil
}

// addEnvironmentInfoBuiltin returns formatted environment information
// (working directory, git status, OS, arch) as additional context. It
// reuses [session.GetEnvironmentInfo] to keep the format identical to the
// previous inline injection.
//
// The working directory comes from the hook input's Cwd, which the
// runtime populates with its configured working directory. If Cwd is
// empty the hook contributes nothing rather than fabricating misleading
// info.
func addEnvironmentInfoBuiltin(_ context.Context, in *hooks.Input, _ []string) (*hooks.Output, error) {
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

// addPromptFilesBuiltin reads each filename in args and joins their
// contents into AdditionalContext. It replaces the inline AddPromptFiles
// loop that lived in pkg/session/session.go's
// buildContextSpecificSystemMessages.
//
// Registered against EventTurnStart so the file contents are re-read
// every turn — important because the user may be editing the prompt file
// during the session and expects the model to pick up the latest text.
//
// Read errors are logged at warn level (one log line per failing file)
// but do not fail the hook; surviving files still contribute their
// content. This matches the previous loop's behavior of silently
// skipping unreadable files.
func addPromptFilesBuiltin(_ context.Context, in *hooks.Input, args []string) (*hooks.Output, error) {
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
	return &hooks.Output{
		HookSpecificOutput: &hooks.HookSpecificOutput{
			HookEventName:     hooks.EventTurnStart,
			AdditionalContext: strings.Join(parts, "\n\n"),
		},
	}, nil
}

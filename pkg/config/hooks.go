package config

import (
	"strings"

	"github.com/docker/docker-agent/pkg/config/latest"
)

// HooksFromCLI builds a HooksConfig from CLI flag values.
// Each string is treated as a shell command to run.
// Empty strings are silently skipped.
func HooksFromCLI(preToolUse, postToolUse, sessionStart, sessionEnd, onUserInput []string) *latest.HooksConfig {
	hooks := &latest.HooksConfig{}

	if len(preToolUse) > 0 {
		var defs []latest.HookDefinition
		for _, cmd := range preToolUse {
			if strings.TrimSpace(cmd) == "" {
				continue
			}
			defs = append(defs, latest.HookDefinition{Type: "command", Command: cmd})
		}
		if len(defs) > 0 {
			hooks.PreToolUse = []latest.HookMatcherConfig{{Hooks: defs}}
		}
	}

	if len(postToolUse) > 0 {
		var defs []latest.HookDefinition
		for _, cmd := range postToolUse {
			if strings.TrimSpace(cmd) == "" {
				continue
			}
			defs = append(defs, latest.HookDefinition{Type: "command", Command: cmd})
		}
		if len(defs) > 0 {
			hooks.PostToolUse = []latest.HookMatcherConfig{{Hooks: defs}}
		}
	}

	for _, cmd := range sessionStart {
		if strings.TrimSpace(cmd) != "" {
			hooks.SessionStart = append(hooks.SessionStart, latest.HookDefinition{Type: "command", Command: cmd})
		}
	}
	for _, cmd := range sessionEnd {
		if strings.TrimSpace(cmd) != "" {
			hooks.SessionEnd = append(hooks.SessionEnd, latest.HookDefinition{Type: "command", Command: cmd})
		}
	}
	for _, cmd := range onUserInput {
		if strings.TrimSpace(cmd) != "" {
			hooks.OnUserInput = append(hooks.OnUserInput, latest.HookDefinition{Type: "command", Command: cmd})
		}
	}

	if hooks.IsEmpty() {
		return nil
	}

	return hooks
}

// MergeHooks merges CLI hooks into an existing HooksConfig.
// CLI hooks are appended after any hooks already defined in the config.
// When both are non-nil and non-empty, a new merged object is returned
// without mutating either input.
func MergeHooks(base, cli *latest.HooksConfig) *latest.HooksConfig {
	if cli == nil || cli.IsEmpty() {
		return base
	}
	if base == nil || base.IsEmpty() {
		return cli
	}

	merged := &latest.HooksConfig{
		PreToolUse:      append(append([]latest.HookMatcherConfig{}, base.PreToolUse...), cli.PreToolUse...),
		PostToolUse:     append(append([]latest.HookMatcherConfig{}, base.PostToolUse...), cli.PostToolUse...),
		SessionStart:    append(append([]latest.HookDefinition{}, base.SessionStart...), cli.SessionStart...),
		TurnStart:       append(append([]latest.HookDefinition{}, base.TurnStart...), cli.TurnStart...),
		BeforeLLMCall:   append(append([]latest.HookDefinition{}, base.BeforeLLMCall...), cli.BeforeLLMCall...),
		AfterLLMCall:    append(append([]latest.HookDefinition{}, base.AfterLLMCall...), cli.AfterLLMCall...),
		SessionEnd:      append(append([]latest.HookDefinition{}, base.SessionEnd...), cli.SessionEnd...),
		OnUserInput:     append(append([]latest.HookDefinition{}, base.OnUserInput...), cli.OnUserInput...),
		Stop:            append(append([]latest.HookDefinition{}, base.Stop...), cli.Stop...),
		Notification:    append(append([]latest.HookDefinition{}, base.Notification...), cli.Notification...),
		OnError:         append(append([]latest.HookDefinition{}, base.OnError...), cli.OnError...),
		OnMaxIterations: append(append([]latest.HookDefinition{}, base.OnMaxIterations...), cli.OnMaxIterations...),
	}
	return merged
}

// CLIHooks returns a HooksConfig derived from the runtime config's CLI hook flags,
// or nil if no hook flags were specified.
func (runConfig *RuntimeConfig) CLIHooks() *latest.HooksConfig {
	return HooksFromCLI(
		runConfig.HookPreToolUse,
		runConfig.HookPostToolUse,
		runConfig.HookSessionStart,
		runConfig.HookSessionEnd,
		runConfig.HookOnUserInput,
	)
}

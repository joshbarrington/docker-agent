// Package builtins contains the stock in-process hook implementations
// shipped with docker-agent: add_date, add_environment_info, and
// add_prompt_files. Each lives in its own file along with its
// registered-name constant; this file holds the shared registration
// and agent-defaults plumbing.
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
	"errors"

	"github.com/docker/docker-agent/pkg/hooks"
)

// Register installs the stock builtin hooks on r.
func Register(r *hooks.Registry) error {
	return errors.Join(
		r.RegisterBuiltin(AddDate, addDate),
		r.RegisterBuiltin(AddEnvironmentInfo, addEnvironmentInfo),
		r.RegisterBuiltin(AddPromptFiles, addPromptFiles),
	)
}

// AgentDefaults captures the agent-level flags that map onto stock
// builtin hook entries. Pass each AgentConfig.AddXxx flag as-is.
type AgentDefaults struct {
	AddDate            bool
	AddEnvironmentInfo bool
	AddPromptFiles     []string
}

// ApplyAgentDefaults appends the stock builtin hook entries implied by
// d to cfg, returning the (possibly mutated) config.
//
// A nil cfg is treated as empty; the returned value is non-nil iff at
// least one hook (user-configured or auto-injected) is present.
func ApplyAgentDefaults(cfg *hooks.Config, d AgentDefaults) *hooks.Config {
	if cfg == nil {
		cfg = &hooks.Config{}
	}
	if d.AddDate {
		cfg.TurnStart = append(cfg.TurnStart, builtinHook(AddDate))
	}
	if len(d.AddPromptFiles) > 0 {
		cfg.TurnStart = append(cfg.TurnStart, builtinHook(AddPromptFiles, d.AddPromptFiles...))
	}
	if d.AddEnvironmentInfo {
		cfg.SessionStart = append(cfg.SessionStart, builtinHook(AddEnvironmentInfo))
	}
	if cfg.IsEmpty() {
		return nil
	}
	return cfg
}

// builtinHook returns a hook entry that dispatches to the named builtin.
func builtinHook(name string, args ...string) hooks.Hook {
	return hooks.Hook{Type: hooks.HookTypeBuiltin, Command: name, Args: args}
}
